package memtable

import (
	"cmp"
	"slices"
	"sync"
	"unsafe"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

type Query = model.StorageQuery

type MemTable struct {
	mu                sync.RWMutex
	data              tableData
	sampleCount       int
	approxBytes       int64
	outOfOrderSamples uint64
	duplicateSamples  uint64
	appendedSamples   uint64
}

type Snapshot struct {
	data        tableData
	sampleCount int
	approxBytes int64
}

type Stats struct {
	Samples           int
	Series            int
	Fields            int
	Columns           int
	Bytes             int64
	OutOfOrderSamples uint64
	DuplicateSamples  uint64
	AppendedSamples   uint64
}

type columnKey struct {
	seriesID uint64
	fieldID  uint32
}

type columnBuffer struct {
	seriesID      uint64
	fieldID       uint32
	fieldType     model.FieldType
	times         []int64
	writeSeqs     []uint64
	floats        []float64
	ints          []int64
	strings       []string
	boolBits      []uint64
	count         int
	memBytes      int64
	lastTimestamp int64
	hasLast       bool
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
	stats := statsFromData(m.data, m.sampleCount, m.approxBytes)
	stats.OutOfOrderSamples = m.outOfOrderSamples
	stats.DuplicateSamples = m.duplicateSamples
	stats.AppendedSamples = m.appendedSamples
	return stats
}

// DisorderRatio 返回当前累计乱序样本占追加样本的比例；无追加时为 0。
func (m *MemTable) DisorderRatio() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.appendedSamples == 0 {
		return 0
	}
	return float64(m.outOfOrderSamples) / float64(m.appendedSamples)
}

// AppendedSamples 返回打开以来追加样本数（含当前 MemTable 生命周期累计）。
func (m *MemTable) AppendedSamples() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.appendedSamples
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

func (m *MemTable) noteAppendOrderLocked(column *columnBuffer, timestamp int64) {
	m.appendedSamples++
	m.sampleCount++
	if !column.hasLast {
		column.lastTimestamp = timestamp
		column.hasLast = true
		return
	}
	if timestamp < column.lastTimestamp {
		m.outOfOrderSamples++
		column.lastTimestamp = timestamp
		return
	}
	if timestamp == column.lastTimestamp {
		m.duplicateSamples++
		return
	}
	column.lastTimestamp = timestamp
}

func (m *MemTable) applyField(point model.ResolvedPoint, field model.ResolvedField) {
	column, delta := ensureColumn(m.data, point.SeriesID, field.FieldID, field.Type)
	delta += column.appendSample(model.VersionedSample{
		Timestamp: point.Timestamp,
		WriteSeq:  point.WriteSeq,
		Value:     field.Value,
	})
	m.noteAppendOrderLocked(column, point.Timestamp)
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
		m.noteAppendOrderLocked(column, batch.Timestamps[row])
		m.approxBytes += delta
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
	if src.hasLast {
		dst.lastTimestamp = src.lastTimestamp
		dst.hasLast = true
	} else if src.count > 0 {
		dst.lastTimestamp = src.times[src.count-1]
		dst.hasLast = true
	}
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
