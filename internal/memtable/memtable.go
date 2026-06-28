package memtable

import (
	"cmp"
	"slices"
	"sort"
	"sync"
	"unsafe"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type Query = model.StorageQuery

type MemTable struct {
	mu          sync.RWMutex
	data        tableData
	sampleCount int
	approxBytes int64
}

type Snapshot struct {
	data        tableData
	sampleCount int
	approxBytes int64
}

type Stats struct {
	Samples int
	Series  int
	Fields  int
	Columns int
	Bytes   int64
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
	memBytes  int64
}

type tableData map[columnKey]*columnBuffer

type columnReservation struct {
	fieldType model.FieldType
	count     int
}

const maxPooledReservations = 1 << 15

const uniqueSeriesReservationBypassMinPoints = 128

const (
	tableDataBaseBytes    = int64(unsafe.Sizeof(tableData{}))
	mapEntryApproxBytes   = int64(unsafe.Sizeof(columnKey{}) + unsafe.Sizeof(&columnBuffer{}))
	columnBufferBaseBytes = int64(unsafe.Sizeof(columnBuffer{}))
	int64Bytes            = int64(unsafe.Sizeof(int64(0)))
	uint64Bytes           = int64(unsafe.Sizeof(uint64(0)))
	float64Bytes          = int64(unsafe.Sizeof(float64(0)))
	stringHeaderBytes     = int64(unsafe.Sizeof(""))
)

var reservationMapPool = sync.Pool{
	New: func() any {
		reservations := make(map[columnKey]columnReservation)
		return &reservations
	},
}

var cloneDataHook func()

var approxDataHook func()

var borrowReservationMapHook func()

func New() *MemTable {
	return &MemTable{
		data:        borrowTableData(),
		approxBytes: tableDataBaseBytes,
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

func (m *MemTable) ApplyTypedBatch(batch model.ResolvedTypedBatch, rows []int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reserveTypedBatchLocked(batch, rows)
	for position := range typedBatchRowCount(batch, rows) {
		m.applyTypedRowLocked(batch, typedBatchRowIndex(rows, position))
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
	return m.approxBytes
}

func (m *MemTable) StatsSnapshot() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return statsFromData(m.data, m.sampleCount, m.approxBytes)
}

func (m *MemTable) SnapshotAndReset() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := &Snapshot{
		data:        m.data,
		sampleCount: m.sampleCount,
		approxBytes: m.approxBytes,
	}
	m.data = borrowTableData()
	m.sampleCount = 0
	m.approxBytes = tableDataBaseBytes
	return snapshot
}

func (m *MemTable) Snapshot() *Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data := cloneData(m.data)
	return &Snapshot{
		data:        data,
		sampleCount: m.sampleCount,
		approxBytes: trackedTableDataBytes(data),
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

func (s *Snapshot) ForEachSeries(
	query Query,
	fn func(seriesID uint64, columns []model.ColumnData) error,
) error {
	if s == nil || query.End < query.Start {
		return nil
	}
	if err := queryContextErr(query); err != nil {
		return err
	}
	keys := sortedColumnKeys(s.data, query)
	defer releaseColumnKeys(keys)
	columns := make([]model.ColumnData, 0, 8)
	samples := make([][]model.VersionedSample, 0, 8)
	for start := 0; start < len(keys); {
		seriesID := keys[start].seriesID
		end := start + 1
		for end < len(keys) && keys[end].seriesID == seriesID {
			end++
		}
		columns, samples = seriesColumnsFromKeys(s.data, keys[start:end], query, columns, samples)
		if len(columns) > 0 {
			if err := fn(seriesID, columns); err != nil {
				return err
			}
		}
		if err := queryContextErr(query); err != nil {
			return err
		}
		start = end
	}
	return nil
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

func sortedColumnKeys(data tableData, query Query) []columnKey {
	keys := borrowColumnKeys(len(data))
	for key, column := range data {
		if column == nil || !containsSeries(query.SeriesIDs, column.seriesID) {
			continue
		}
		if !containsField(query.FieldIDs, column.fieldID) {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left columnKey, right columnKey) int {
		if left.seriesID != right.seriesID {
			return cmp.Compare(left.seriesID, right.seriesID)
		}
		return cmp.Compare(left.fieldID, right.fieldID)
	})
	return keys
}

func seriesColumnsFromKeys(
	data tableData,
	keys []columnKey,
	query Query,
	columns []model.ColumnData,
	samples [][]model.VersionedSample,
) ([]model.ColumnData, [][]model.VersionedSample) {
	columns = columns[:0]
	if cap(samples) < len(keys) {
		samples = make([][]model.VersionedSample, len(keys))
	} else {
		samples = samples[:len(keys)]
	}
	for index, key := range keys {
		column := data[key]
		if column == nil {
			continue
		}
		columnSamples := compactSamplesInto(samples[index][:0], column, query)
		samples[index] = columnSamples
		if len(columnSamples) > 0 {
			columns = append(columns, model.ColumnData{
				SeriesID:  column.seriesID,
				FieldID:   column.fieldID,
				FieldType: column.fieldType,
				Samples:   columnSamples,
			})
		}
	}
	return columns, samples
}

func queryContextErr(query Query) error {
	if query.Context == nil {
		return nil
	}
	return query.Context.Err()
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
	return s.approxBytes
}

func (s *Snapshot) StatsSnapshot() Stats {
	if s == nil {
		return Stats{}
	}
	return statsFromData(s.data, s.sampleCount, s.approxBytes)
}

func statsFromData(data tableData, samples int, bytes int64) Stats {
	series := make(map[uint64]struct{})
	fields := make(map[uint32]struct{})
	for key := range data {
		series[key.seriesID] = struct{}{}
		fields[key.fieldID] = struct{}{}
	}
	return Stats{
		Samples: samples,
		Series:  len(series),
		Fields:  len(fields),
		Columns: len(data),
		Bytes:   bytes,
	}
}

func (s *Snapshot) Release() {
	if s == nil {
		return
	}
	data := s.data
	for _, column := range s.data {
		releaseColumnBuffer(column)
	}
	releaseTableData(data)
	s.data = nil
	s.sampleCount = 0
	s.approxBytes = 0
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
	if !shouldReserveBatch(points) {
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
		column, delta := ensureColumn(m.data, key.seriesID, key.fieldID, reservation.fieldType)
		delta += column.reserve(reservation.count)
		m.approxBytes += delta
	}
	releaseReservationMap(reservations)
}

func (m *MemTable) reserveTypedBatchLocked(batch model.ResolvedTypedBatch, rows []int) {
	if !shouldReserveTypedBatch(batch, rows) {
		return
	}
	reservations := borrowReservationMap()
	for position := range typedBatchRowCount(batch, rows) {
		row := typedBatchRowIndex(rows, position)
		for _, field := range batch.Fields {
			key := columnKey{seriesID: batch.SeriesIDs[row], fieldID: field.FieldID}
			reservation := reservations[key]
			reservation.fieldType = field.Type
			reservation.count++
			reservations[key] = reservation
		}
	}
	for key, reservation := range reservations {
		column, delta := ensureColumn(m.data, key.seriesID, key.fieldID, reservation.fieldType)
		delta += column.reserve(reservation.count)
		m.approxBytes += delta
	}
	releaseReservationMap(reservations)
}

func shouldReserveBatch(points []model.ResolvedPoint) bool {
	if len(points) <= 1 {
		return false
	}
	if len(points) < uniqueSeriesReservationBypassMinPoints {
		return true
	}
	return !hasUniqueSeriesPrefix(points, uniqueSeriesReservationBypassMinPoints)
}

func hasUniqueSeriesPrefix(points []model.ResolvedPoint, limit int) bool {
	if limit > len(points) {
		limit = len(points)
	}
	for index := 0; index < limit; index++ {
		seriesID := points[index].SeriesID
		for previous := 0; previous < index; previous++ {
			if points[previous].SeriesID == seriesID {
				return false
			}
		}
	}
	return true
}

func shouldReserveTypedBatch(batch model.ResolvedTypedBatch, rows []int) bool {
	count := typedBatchRowCount(batch, rows)
	if count <= 1 {
		return false
	}
	if count < uniqueSeriesReservationBypassMinPoints {
		return true
	}
	return !hasUniqueTypedSeriesPrefix(batch, rows, uniqueSeriesReservationBypassMinPoints)
}

func hasUniqueTypedSeriesPrefix(
	batch model.ResolvedTypedBatch,
	rows []int,
	limit int,
) bool {
	if limit > typedBatchRowCount(batch, rows) {
		limit = typedBatchRowCount(batch, rows)
	}
	for index := 0; index < limit; index++ {
		seriesID := batch.SeriesIDs[typedBatchRowIndex(rows, index)]
		for previous := 0; previous < index; previous++ {
			if batch.SeriesIDs[typedBatchRowIndex(rows, previous)] == seriesID {
				return false
			}
		}
	}
	return true
}

func borrowReservationMap() map[columnKey]columnReservation {
	if borrowReservationMapHook != nil {
		borrowReservationMapHook()
	}
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
	column, delta := ensureColumn(m.data, point.SeriesID, field.FieldID, field.Type)
	delta += column.appendSample(model.VersionedSample{
		Timestamp: point.Timestamp,
		WriteSeq:  point.WriteSeq,
		Value:     field.Value,
	})
	m.approxBytes += delta
}

func (m *MemTable) applyTypedRowLocked(batch model.ResolvedTypedBatch, row int) {
	for _, field := range batch.Fields {
		column, delta := ensureColumn(m.data, batch.SeriesIDs[row], field.FieldID, field.Type)
		delta += column.appendSample(model.VersionedSample{
			Timestamp: batch.Timestamps[row],
			WriteSeq:  batch.WriteSeqs[row],
			Value:     typedFieldValueAt(field, row),
		})
		m.approxBytes += delta
		m.sampleCount++
	}
}

func (m *MemTable) restoreColumnLocked(src *columnBuffer) {
	if src == nil || src.count == 0 {
		return
	}
	dst, delta := ensureColumn(m.data, src.seriesID, src.fieldID, src.fieldType)
	delta += dst.appendColumn(src)
	m.approxBytes += delta
	m.sampleCount += src.count
}

func ensureColumn(
	data tableData,
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
) (*columnBuffer, int64) {
	key := columnKey{
		seriesID: seriesID,
		fieldID:  fieldID,
	}
	column, ok := data[key]
	if !ok {
		column = borrowColumnBuffer(seriesID, fieldID, fieldType)
		data[key] = column
		return column, mapEntryApproxBytes + column.memBytes
	}
	column.fieldType = fieldType
	return column, 0
}

func columnDataFromBuffer(column *columnBuffer, query Query) model.ColumnData {
	return model.ColumnData{
		SeriesID:  column.seriesID,
		FieldID:   column.fieldID,
		FieldType: column.fieldType,
		Samples:   compactSamples(column, query),
	}
}

func (c *columnBuffer) appendSample(sample model.VersionedSample) int64 {
	if c.fieldType == 0 {
		c.fieldType = sample.Value.Type
	}
	delta := c.reserve(1)
	appendDelta := int64(0)
	oldTimesCap := cap(c.times)
	c.times = append(c.times, sample.Timestamp)
	appendDelta += int64(cap(c.times)-oldTimesCap) * int64Bytes
	oldSeqsCap := cap(c.writeSeqs)
	c.writeSeqs = append(c.writeSeqs, sample.WriteSeq)
	appendDelta += int64(cap(c.writeSeqs)-oldSeqsCap) * uint64Bytes
	appendDelta += c.appendValue(sample.Value)
	c.count++
	c.memBytes += appendDelta
	return delta + appendDelta
}

func (c *columnBuffer) appendValue(value model.FieldValue) int64 {
	switch c.fieldType {
	case model.FieldFloat64:
		oldCap := cap(c.floats)
		c.floats = append(c.floats, value.Float64)
		return int64(cap(c.floats)-oldCap) * float64Bytes
	case model.FieldInt64:
		oldCap := cap(c.ints)
		c.ints = append(c.ints, value.Int64)
		return int64(cap(c.ints)-oldCap) * int64Bytes
	case model.FieldString:
		oldCap := cap(c.strings)
		c.strings = append(c.strings, value.String)
		return int64(cap(c.strings)-oldCap)*stringHeaderBytes + int64(len(value.String))
	case model.FieldBool:
		return c.appendBool(value.Bool)
	}
	return 0
}

func (c *columnBuffer) appendBool(value bool) int64 {
	word := c.count / 64
	oldCap := cap(c.boolBits)
	if word >= len(c.boolBits) {
		c.boolBits = append(c.boolBits, 0)
	}
	if value {
		c.boolBits[word] |= 1 << uint(c.count%64)
	}
	return int64(cap(c.boolBits)-oldCap) * uint64Bytes
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

func (c *columnBuffer) reserve(additional int) int64 {
	if additional <= 0 {
		return 0
	}
	before := c.capacityBytes()
	target := c.count + additional
	c.times = growInt64s(c.times, target, additional)
	c.writeSeqs = growUint64s(c.writeSeqs, target, additional)
	c.reserveValues(target, additional)
	delta := c.capacityBytes() - before
	c.memBytes += delta
	return delta
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
	if approxDataHook != nil {
		approxDataHook()
	}
	if data == nil {
		return 0
	}
	total := tableDataBaseBytes + int64(len(data))*mapEntryApproxBytes
	for _, column := range data {
		if column != nil {
			total += column.approxMemoryBytes()
		}
	}
	return total
}

func trackedTableDataBytes(data tableData) int64 {
	if data == nil {
		return 0
	}
	total := tableDataBaseBytes + int64(len(data))*mapEntryApproxBytes
	for _, column := range data {
		if column != nil {
			total += column.memBytes
		}
	}
	return total
}

func (c *columnBuffer) approxMemoryBytes() int64 {
	if c == nil {
		return 0
	}
	total := columnBufferBaseBytes + c.capacityBytes()
	for _, value := range c.strings[:min(c.count, len(c.strings))] {
		total += int64(len(value))
	}
	return total
}

func (c *columnBuffer) capacityBytes() int64 {
	if c == nil {
		return 0
	}
	total := int64(cap(c.times)) * int64Bytes
	total += int64(cap(c.writeSeqs)) * uint64Bytes
	total += int64(cap(c.floats)) * float64Bytes
	total += int64(cap(c.ints)) * int64Bytes
	total += int64(cap(c.strings)) * stringHeaderBytes
	total += int64(cap(c.boolBits)) * uint64Bytes
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

func typedBatchRowCount(batch model.ResolvedTypedBatch, rows []int) int {
	if len(rows) > 0 {
		return len(rows)
	}
	return len(batch.Timestamps)
}

func typedBatchRowIndex(rows []int, position int) int {
	if len(rows) > 0 {
		return rows[position]
	}
	return position
}

func typedFieldValueAt(field model.ResolvedTypedFieldColumn, row int) model.FieldValue {
	switch field.Type {
	case model.FieldFloat64:
		return model.Float64Value(field.Float64Values[row])
	case model.FieldInt64:
		return model.Int64Value(field.Int64Values[row])
	case model.FieldString:
		return model.StringValue(field.StringValues[row])
	case model.FieldBool:
		return model.BoolValue(field.BoolValues[row])
	default:
		return model.FieldValue{Type: field.Type}
	}
}

func (c *columnBuffer) appendColumn(src *columnBuffer) int64 {
	if src.count == 0 {
		return 0
	}
	if c.fieldType == 0 {
		c.fieldType = src.fieldType
	}
	delta := c.reserve(src.count)
	for index := range src.count {
		delta += c.appendSample(src.sampleAt(index))
	}
	return delta
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

func compactSamplesInto(
	dst []model.VersionedSample,
	column *columnBuffer,
	query Query,
) []model.VersionedSample {
	if column.count == 0 {
		return dst[:0]
	}
	if columnTimesSortedUnique(column) {
		return materializeSortedColumnSamplesInto(dst, column, query)
	}
	matches := countMatchingSamples(column, query)
	if matches == 0 {
		return dst[:0]
	}
	if cap(dst) < matches {
		dst = make([]model.VersionedSample, 0, matches)
	} else {
		dst = dst[:0]
	}
	for index := range column.count {
		sample := column.sampleAt(index)
		dst = appendMatchingSample(dst, sample, query)
	}
	return compactMaterializedSamples(dst)
}

func materializeSortedColumnSamples(column *columnBuffer, query Query) []model.VersionedSample {
	start, end := sortedRangeBounds(column.times[:column.count], query)
	samples := make([]model.VersionedSample, end-start)
	for index := start; index < end; index++ {
		samples[index-start] = column.sampleAt(index)
	}
	return samples
}

func materializeSortedColumnSamplesInto(
	dst []model.VersionedSample,
	column *columnBuffer,
	query Query,
) []model.VersionedSample {
	start, end := sortedRangeBounds(column.times[:column.count], query)
	needed := end - start
	if cap(dst) < needed {
		dst = make([]model.VersionedSample, needed)
	} else {
		dst = dst[:needed]
	}
	for index := start; index < end; index++ {
		dst[index-start] = column.sampleAt(index)
	}
	return dst
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
	slices.SortFunc(samples, func(left model.VersionedSample, right model.VersionedSample) int {
		if left.Timestamp != right.Timestamp {
			return cmp.Compare(left.Timestamp, right.Timestamp)
		}
		return cmp.Compare(right.WriteSeq, left.WriteSeq)
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
	slices.SortFunc(columns, func(left model.ColumnData, right model.ColumnData) int {
		if left.SeriesID != right.SeriesID {
			return cmp.Compare(left.SeriesID, right.SeriesID)
		}
		return cmp.Compare(left.FieldID, right.FieldID)
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
	clone := &columnBuffer{
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
	clone.memBytes = clone.approxMemoryBytes()
	return clone
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
