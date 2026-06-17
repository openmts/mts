package memtable

import (
	"context"
	"sort"
	"sync"
	"unsafe"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/queryexec"
)

type Query struct {
	Context   context.Context
	Budget    model.QueryBudget
	Stats     *model.QueryStats
	Boundary  model.QueryBoundaryMode
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
	times     []int64
	writeSeqs []uint64
	floats    []float64
	ints      []int64
	strings   []string
	boolBits  []uint64
	count     int
}

type tableData map[columnKey]*columnBuffer

type columnReservation struct {
	fieldType model.FieldType
	count     int
}

const maxPooledReservations = 1 << 15

const (
	tableDataBaseBytes  = int(unsafe.Sizeof(tableData{}))
	mapEntryApproxBytes = int(unsafe.Sizeof(columnKey{}) + unsafe.Sizeof(&columnBuffer{}))
)

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

func (m *MemTable) ApproxMemoryBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return approxTableDataBytes(m.data)
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
	return columnsFromData(m.data, query)
}

func (m *MemTable) ScanColumns(query Query) queryexec.ColumnDataStream {
	if query.Context != nil {
		if err := query.Context.Err(); err != nil {
			return queryexec.NewErrorColumnDataStream(err)
		}
	}
	return queryexec.WithContextColumnDataStream(query.Context, queryexec.NewSliceColumnDataStream(m.Query(query)))
}

func (s *Snapshot) Query(query Query) []model.ColumnData {
	return s.Columns(query)
}

func (s *Snapshot) Columns(query Query) []model.ColumnData {
	if s == nil {
		return []model.ColumnData{}
	}
	return columnsFromData(s.data, query)
}

func columnsFromData(data tableData, query Query) []model.ColumnData {
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
		out := columnDataFromBuffer(column, query)
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

func (s *Snapshot) ApproxMemoryBytes() int64 {
	if s == nil {
		return 0
	}
	return approxTableDataBytes(s.data)
}

func (s *Snapshot) Release() {
	if s == nil {
		return
	}
	data := s.data
	for _, column := range s.data {
		if column != nil {
			column.clear()
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

func columnDataFromBuffer(column *columnBuffer, query Query) model.ColumnData {
	return model.ColumnData{
		SeriesID:  column.seriesID,
		FieldID:   column.fieldID,
		FieldType: column.fieldType,
		Samples:   compactSamples(column, query),
	}
}

func (c *columnBuffer) appendSample(sample model.VersionedSample) {
	if c.fieldType == 0 {
		c.fieldType = sample.Value.Type
	}
	c.times = append(c.times, sample.Timestamp)
	c.writeSeqs = append(c.writeSeqs, sample.WriteSeq)
	c.appendValue(sample.Value)
	c.count++
}

func (c *columnBuffer) appendValue(value model.FieldValue) {
	switch c.fieldType {
	case model.FieldFloat64:
		c.floats = append(c.floats, value.Float64)
	case model.FieldInt64:
		c.ints = append(c.ints, value.Int64)
	case model.FieldString:
		c.strings = append(c.strings, value.String)
	case model.FieldBool:
		c.appendBool(value.Bool)
	}
}

func (c *columnBuffer) appendBool(value bool) {
	word := c.count / 64
	if word >= len(c.boolBits) {
		c.boolBits = append(c.boolBits, 0)
	}
	if value {
		c.boolBits[word] |= 1 << uint(c.count%64)
	}
}

func (c *columnBuffer) sampleAt(index int) model.VersionedSample {
	return model.VersionedSample{
		Timestamp: c.times[index],
		WriteSeq:  c.writeSeqs[index],
		Value:     c.valueAt(index),
	}
}

func (c *columnBuffer) valueAt(index int) model.FieldValue {
	switch c.fieldType {
	case model.FieldFloat64:
		return model.Float64Value(c.floats[index])
	case model.FieldInt64:
		return model.Int64Value(c.ints[index])
	case model.FieldString:
		return model.StringValue(c.strings[index])
	case model.FieldBool:
		word := index / 64
		bit := uint(index % 64)
		return model.BoolValue(word < len(c.boolBits) && c.boolBits[word]&(1<<bit) != 0)
	default:
		return model.FieldValue{Type: c.fieldType}
	}
}

func (c *columnBuffer) reserve(additional int) {
	if additional <= 0 {
		return
	}
	target := c.count + additional
	c.times = growInt64s(c.times, target, additional)
	c.writeSeqs = growUint64s(c.writeSeqs, target, additional)
	c.reserveValues(target, additional)
}

func (c *columnBuffer) reserveValues(target int, additional int) {
	switch c.fieldType {
	case model.FieldFloat64:
		c.floats = growFloat64s(c.floats, target, additional)
	case model.FieldInt64:
		c.ints = growInt64s(c.ints, target, additional)
	case model.FieldString:
		c.strings = growStrings(c.strings, target, additional)
	case model.FieldBool:
		c.boolBits = growUint64s(c.boolBits, boolWords(target), boolWords(additional))
	}
}

func (c *columnBuffer) clear() {
	c.times = nil
	c.writeSeqs = nil
	c.floats = nil
	c.ints = nil
	c.strings = nil
	c.boolBits = nil
}

func approxTableDataBytes(data tableData) int64 {
	if data == nil {
		return 0
	}
	total := int64(tableDataBaseBytes + len(data)*mapEntryApproxBytes)
	for _, column := range data {
		if column != nil {
			total += column.approxMemoryBytes()
		}
	}
	return total
}

func (c *columnBuffer) approxMemoryBytes() int64 {
	if c == nil {
		return 0
	}
	total := int64(unsafe.Sizeof(*c))
	total += int64(cap(c.times)) * int64(unsafe.Sizeof(int64(0)))
	total += int64(cap(c.writeSeqs)) * int64(unsafe.Sizeof(uint64(0)))
	total += int64(cap(c.floats)) * int64(unsafe.Sizeof(float64(0)))
	total += int64(cap(c.ints)) * int64(unsafe.Sizeof(int64(0)))
	total += int64(cap(c.strings)) * int64(unsafe.Sizeof(""))
	total += int64(cap(c.boolBits)) * int64(unsafe.Sizeof(uint64(0)))
	for _, value := range c.strings[:min(c.count, len(c.strings))] {
		total += int64(len(value))
	}
	return total
}

func growInt64s(values []int64, target int, additional int) []int64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]int64, len(values), next)
	copy(out, values)
	return out
}

func growUint64s(values []uint64, target int, additional int) []uint64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]uint64, len(values), next)
	copy(out, values)
	return out
}

func growFloat64s(values []float64, target int, additional int) []float64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]float64, len(values), next)
	copy(out, values)
	return out
}

func growStrings(values []string, target int, additional int) []string {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]string, len(values), next)
	copy(out, values)
	return out
}

func nextSliceCapacity(current int, target int, additional int) int {
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

func boolWords(count int) int {
	if count <= 0 {
		return 0
	}
	return (count + 63) / 64
}

func (c *columnBuffer) appendColumn(src *columnBuffer) {
	if src.count == 0 {
		return
	}
	if c.fieldType == 0 {
		c.fieldType = src.fieldType
	}
	c.reserve(src.count)
	for index := range src.count {
		c.appendSample(src.sampleAt(index))
	}
}

func compactSamples(column *columnBuffer, query Query) []model.VersionedSample {
	if column.count == 0 {
		return []model.VersionedSample{}
	}
	if columnTimesSortedUnique(column) {
		return materializeSortedColumnSamples(column, query)
	}
	matches := countMatchingSamples(column, query)
	if matches == 0 {
		return []model.VersionedSample{}
	}
	samples := make([]model.VersionedSample, 0, matches)
	for index := range column.count {
		sample := column.sampleAt(index)
		samples = appendMatchingSample(samples, sample, query)
	}
	return compactMaterializedSamples(samples)
}

func materializeSortedColumnSamples(column *columnBuffer, query Query) []model.VersionedSample {
	start, end := sortedRangeBounds(column.times[:column.count], query)
	samples := make([]model.VersionedSample, end-start)
	for index := start; index < end; index++ {
		samples[index-start] = column.sampleAt(index)
	}
	return samples
}

func sortedRangeBounds(times []int64, query Query) (int, int) {
	start := sort.Search(len(times), func(index int) bool {
		return times[index] >= query.Start
	})
	end := sort.Search(len(times), func(index int) bool {
		return times[index] > query.End
	})
	if end < start {
		return start, start
	}
	return start, end
}

func columnTimesSortedUnique(column *columnBuffer) bool {
	var previous int64
	for index := range column.count {
		timestamp := column.times[index]
		if index > 0 && timestamp <= previous {
			return false
		}
		previous = timestamp
	}
	return true
}

func countMatchingSamples(column *columnBuffer, query Query) int {
	count := 0
	for index := range column.count {
		timestamp := column.times[index]
		if timestamp >= query.Start && timestamp <= query.End {
			count++
		}
	}
	return count
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
		times:     cloneInt64s(src.times),
		writeSeqs: cloneUint64s(src.writeSeqs),
		floats:    cloneFloat64s(src.floats),
		ints:      cloneInt64s(src.ints),
		strings:   cloneStrings(src.strings),
		boolBits:  cloneUint64s(src.boolBits),
		count:     src.count,
	}
}

func cloneInt64s(src []int64) []int64 {
	dst := make([]int64, len(src))
	copy(dst, src)
	return dst
}

func cloneUint64s(src []uint64) []uint64 {
	dst := make([]uint64, len(src))
	copy(dst, src)
	return dst
}

func cloneFloat64s(src []float64) []float64 {
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}

func cloneStrings(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
