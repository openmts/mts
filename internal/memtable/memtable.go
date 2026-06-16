package memtable

import (
	"sort"
	"sync"

	"codeberg.org/mts/mts/internal/model"
)

type Query struct {
	SeriesIDs map[uint64]struct{}
	FieldIDs  map[uint32]struct{}
	Start     int64
	End       int64
}

type MemTable struct {
	mu          sync.RWMutex
	data        tableData
	sampleCount int
}

type Snapshot struct {
	data        tableData
	sampleCount int
}

type columnKey struct {
	seriesID uint64
	fieldID  uint32
}

type columnBuffer struct {
	seriesID  uint64
	fieldID   uint32
	fieldType model.FieldType
	first     model.VersionedSample
	samples   []model.VersionedSample
	count     int
}

type tableData map[columnKey]*columnBuffer

type columnReservation struct {
	fieldType model.FieldType
	count     int
}

const maxPooledReservations = 1 << 15

var reservationMapPool = sync.Pool{
	New: func() any {
		reservations := make(map[columnKey]columnReservation)
		return &reservations
	},
}

var cloneDataHook func()

func New() *MemTable {
	return &MemTable{
		data: borrowTableData(),
	}
}

func (m *MemTable) Apply(point model.ResolvedPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applyPointLocked(point)
	return nil
}

func (m *MemTable) ApplyBatch(points []model.ResolvedPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reserveBatchLocked(points)
	for _, point := range points {
		m.applyPointLocked(point)
	}
	return nil
}

func (m *MemTable) SampleCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sampleCount
}

func (m *MemTable) SnapshotAndReset() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := &Snapshot{
		data:        m.data,
		sampleCount: m.sampleCount,
	}
	m.data = borrowTableData()
	m.sampleCount = 0
	return snapshot
}

func (m *MemTable) Snapshot() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &Snapshot{
		data:        cloneData(m.data),
		sampleCount: m.sampleCount,
	}
}

func (m *MemTable) Query(query Query) []model.ColumnData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return columnsFromData(m.data, query, true)
}

func (s *Snapshot) Query(query Query) []model.ColumnData {
	return s.Columns(query)
}

func (s *Snapshot) Columns(query Query) []model.ColumnData {
	if s == nil {
		return []model.ColumnData{}
	}
	return columnsFromData(s.data, query, false)
}

func columnsFromData(data tableData, query Query, detach bool) []model.ColumnData {
	if query.End < query.Start {
		return []model.ColumnData{}
	}
	columns := make([]model.ColumnData, 0, len(data))
	for _, column := range data {
		if column == nil || !containsSeries(query.SeriesIDs, column.seriesID) {
			continue
		}
		if !containsField(query.FieldIDs, column.fieldID) {
			continue
		}
		out := columnDataFromBuffer(column, query, detach)
		if len(out.Samples) > 0 {
			columns = append(columns, out)
		}
	}
	sortColumns(columns)
	return columns
}

func (s *Snapshot) SampleCount() int {
	if s == nil {
		return 0
	}
	return s.sampleCount
}

func (s *Snapshot) Release() {
	if s == nil {
		return
	}
	data := s.data
	for _, column := range s.data {
		if column != nil {
			column.first = model.VersionedSample{}
			column.samples = nil
			column.count = 0
		}
	}
	releaseTableData(data)
	s.data = nil
	s.sampleCount = 0
}

func (m *MemTable) Restore(snapshot *Snapshot) {
	if snapshot == nil || snapshot.sampleCount == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, column := range snapshot.data {
		m.restoreColumnLocked(column)
	}
}

func (m *MemTable) applyPointLocked(point model.ResolvedPoint) {
	for _, field := range point.Fields {
		m.applyField(point, field)
		m.sampleCount++
	}
}

func (m *MemTable) reserveBatchLocked(points []model.ResolvedPoint) {
	if len(points) <= 1 {
		return
	}
	reservations := borrowReservationMap()
	for _, point := range points {
		for _, field := range point.Fields {
			key := columnKey{seriesID: point.SeriesID, fieldID: field.FieldID}
			reservation := reservations[key]
			reservation.fieldType = field.Type
			reservation.count++
			reservations[key] = reservation
		}
	}
	for key, reservation := range reservations {
		column := ensureColumn(m.data, key.seriesID, key.fieldID, reservation.fieldType)
		column.reserve(reservation.count)
	}
	releaseReservationMap(reservations)
}

func borrowReservationMap() map[columnKey]columnReservation {
	ptr, ok := reservationMapPool.Get().(*map[columnKey]columnReservation)
	if !ok || ptr == nil {
		return make(map[columnKey]columnReservation)
	}
	reservations := *ptr
	clear(reservations)
	return reservations
}

func releaseReservationMap(reservations map[columnKey]columnReservation) {
	if reservations == nil {
		return
	}
	if len(reservations) > maxPooledReservations {
		return
	}
	reservationMapPool.Put(&reservations)
}

func (m *MemTable) applyField(point model.ResolvedPoint, field model.ResolvedField) {
	column := ensureColumn(m.data, point.SeriesID, field.FieldID, field.Type)
	column.appendSample(model.VersionedSample{
		Timestamp: point.Timestamp,
		WriteSeq:  point.WriteSeq,
		Value:     field.Value,
	})
}

func (m *MemTable) restoreColumnLocked(src *columnBuffer) {
	if src == nil || src.count == 0 {
		return
	}
	dst := ensureColumn(m.data, src.seriesID, src.fieldID, src.fieldType)
	dst.appendColumn(src)
	m.sampleCount += src.count
}

func ensureColumn(
	data tableData,
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
) *columnBuffer {
	key := columnKey{
		seriesID: seriesID,
		fieldID:  fieldID,
	}
	column, ok := data[key]
	if !ok {
		column = &columnBuffer{
			seriesID:  seriesID,
			fieldID:   fieldID,
			fieldType: fieldType,
		}
		data[key] = column
		return column
	}
	column.fieldType = fieldType
	return column
}

func columnDataFromBuffer(column *columnBuffer, query Query, detach bool) model.ColumnData {
	return model.ColumnData{
		SeriesID:  column.seriesID,
		FieldID:   column.fieldID,
		FieldType: column.fieldType,
		Samples:   compactSamples(column, query, detach),
	}
}

func (c *columnBuffer) appendSample(sample model.VersionedSample) {
	if c.count == 0 && cap(c.samples) == 0 {
		c.first = sample
		c.count = 1
		return
	}
	if c.count == 1 && len(c.samples) == 0 {
		c.samples = append(c.samples, c.first, sample)
		c.first = model.VersionedSample{}
		c.count = 2
		return
	}
	c.samples = append(c.samples, sample)
	c.count++
}

func (c *columnBuffer) reserve(additional int) {
	if additional <= 0 {
		return
	}
	target := len(c.samples) + additional
	if c.count == 0 && additional == 1 {
		target--
	}
	if c.count == 1 && len(c.samples) == 0 {
		target++
	}
	if target <= cap(c.samples) {
		return
	}
	samples := make([]model.VersionedSample, len(c.samples), c.nextCapacity(target, additional))
	copy(samples, c.samples)
	c.samples = samples
}

func (c *columnBuffer) nextCapacity(target int, additional int) int {
	current := cap(c.samples)
	if current == 0 || additional > current {
		return target
	}
	slack := current / 4
	if slack < additional {
		slack = additional
	}
	if slack > 8 {
		slack = 8
	}
	next := target + slack
	if next < target {
		return target
	}
	return next
}

func (c *columnBuffer) appendColumn(src *columnBuffer) {
	if src.count == 0 {
		return
	}
	if len(src.samples) != src.count {
		c.appendSample(src.first)
	}
	for _, sample := range src.samples {
		c.appendSample(sample)
	}
}

func compactSamples(column *columnBuffer, query Query, detach bool) []model.VersionedSample {
	if column.count == 0 {
		return []model.VersionedSample{}
	}
	if len(column.samples) == column.count {
		return compactDenseSamples(column.samples, query, detach)
	}
	samples := make([]model.VersionedSample, 0, column.count)
	samples = appendMatchingSample(samples, column.first, query)
	for _, sample := range column.samples {
		samples = appendMatchingSample(samples, sample, query)
	}
	return compactMaterializedSamples(samples)
}

func compactDenseSamples(samples []model.VersionedSample, query Query, detach bool) []model.VersionedSample {
	if denseSamplesInRangeSortedUnique(samples, query) {
		if detach {
			return cloneSamples(samples)
		}
		return samples
	}
	filtered := make([]model.VersionedSample, 0, len(samples))
	for _, sample := range samples {
		filtered = appendMatchingSample(filtered, sample, query)
	}
	return compactMaterializedSamples(filtered)
}

func denseSamplesInRangeSortedUnique(samples []model.VersionedSample, query Query) bool {
	var previous int64
	for index, sample := range samples {
		if sample.Timestamp < query.Start || sample.Timestamp > query.End {
			return false
		}
		if index > 0 && sample.Timestamp <= previous {
			return false
		}
		previous = sample.Timestamp
	}
	return true
}

func compactMaterializedSamples(samples []model.VersionedSample) []model.VersionedSample {
	if len(samples) <= 1 {
		return samples
	}
	sort.Slice(samples, func(i, j int) bool {
		if samples[i].Timestamp != samples[j].Timestamp {
			return samples[i].Timestamp < samples[j].Timestamp
		}
		return samples[i].WriteSeq > samples[j].WriteSeq
	})
	write := 0
	for _, sample := range samples {
		if write > 0 && samples[write-1].Timestamp == sample.Timestamp {
			continue
		}
		samples[write] = sample
		write++
	}
	return samples[:write]
}

func appendMatchingSample(
	dst []model.VersionedSample,
	sample model.VersionedSample,
	query Query,
) []model.VersionedSample {
	if sample.Timestamp < query.Start || sample.Timestamp > query.End {
		return dst
	}
	return append(dst, sample)
}

func sortColumns(columns []model.ColumnData) {
	sort.Slice(columns, func(i, j int) bool {
		if columns[i].SeriesID != columns[j].SeriesID {
			return columns[i].SeriesID < columns[j].SeriesID
		}
		return columns[i].FieldID < columns[j].FieldID
	})
}

func containsSeries(filter map[uint64]struct{}, seriesID uint64) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[seriesID]
	return ok
}

func containsField(filter map[uint32]struct{}, fieldID uint32) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[fieldID]
	return ok
}

func cloneData(src tableData) tableData {
	if cloneDataHook != nil {
		cloneDataHook()
	}
	dst := make(tableData, len(src))
	for key, column := range src {
		dst[key] = cloneColumn(column)
	}
	return dst
}

func cloneColumn(src *columnBuffer) *columnBuffer {
	if src == nil {
		return nil
	}
	return &columnBuffer{
		seriesID:  src.seriesID,
		fieldID:   src.fieldID,
		fieldType: src.fieldType,
		first:     src.first,
		samples:   cloneSamples(src.samples),
		count:     src.count,
	}
}

func cloneSamples(src []model.VersionedSample) []model.VersionedSample {
	dst := make([]model.VersionedSample, len(src))
	copy(dst, src)
	return dst
}
