package memtable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestColumnBufferHelpers(t *testing.T) {
	var dst columnBuffer
	var empty columnBuffer
	dst.appendColumn(&empty)
	if dst.count != 0 {
		t.Fatalf("append empty column count = %d, want 0", dst.count)
	}

	var src columnBuffer
	src.appendSample(model.VersionedSample{
		Timestamp: 10,
		WriteSeq:  1,
		Value:     model.Float64Value(1),
	})
	src.appendSample(model.VersionedSample{
		Timestamp: 20,
		WriteSeq:  2,
		Value:     model.Float64Value(2),
	})
	dst.appendColumn(&src)
	if dst.count != 2 {
		t.Fatalf("append column count = %d, want 2", dst.count)
	}

	if cloneColumn(nil) != nil {
		t.Fatal("cloneColumn(nil) != nil")
	}
	cloned := cloneColumn(&dst)
	cloned.times[0] = 99
	cloned.floats[0] = 99
	if dst.times[0] == 99 || dst.floats[0] == 99 {
		t.Fatal("cloneColumn() shared typed backing array")
	}

	filtered := appendMatchingSample(nil, model.VersionedSample{Timestamp: 5}, Query{
		Start: 10,
		End:   20,
	})
	if len(filtered) != 0 {
		t.Fatalf("filtered sample count = %d, want 0", len(filtered))
	}

	dst.reserve(0)
	dst.reserve(1)
}

func TestApplyBatchReservesColumnCapacity(t *testing.T) {
	mt := New()
	points := []model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(1, 20, 2, model.Float64Value(2)),
		memtableResolvedPoint(1, 30, 3, model.Float64Value(3)),
		memtableResolvedPoint(1, 40, 4, model.Float64Value(4)),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	key := columnKey{seriesID: 1, fieldID: 2}
	column := mt.data[key]
	if column == nil {
		t.Fatal("column was not created")
	}
	if column.count != len(points) {
		t.Fatalf("column count = %d, want %d", column.count, len(points))
	}
	if cap(column.times) < len(points) {
		t.Fatalf("column time cap = %d, want at least %d", cap(column.times), len(points))
	}
	if len(column.times) != len(points) || len(column.floats) != len(points) {
		t.Fatalf("typed lens time=%d float=%d, want %d", len(column.times), len(column.floats), len(points))
	}
}

func TestColumnBufferReserveGrowsForSmallIncrementalBatches(t *testing.T) {
	column := columnBuffer{fieldType: model.FieldFloat64}
	column.reserve(8)
	firstCapacity := cap(column.times)
	if firstCapacity < 8 {
		t.Fatalf("first capacity = %d, want at least 8", firstCapacity)
	}
	for index := range 8 {
		column.appendSample(model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		})
	}
	column.reserve(1)
	if cap(column.times) <= firstCapacity {
		t.Fatalf("grown capacity = %d, want greater than %d", cap(column.times), firstCapacity)
	}
	if cap(column.times) > firstCapacity+3 {
		t.Fatalf("grown capacity = %d, want bounded slack over %d", cap(column.times), firstCapacity)
	}
}

func TestColumnBufferStoresStringAndBoolColumnarValues(t *testing.T) {
	var column columnBuffer
	first := model.VersionedSample{
		Timestamp: 1,
		WriteSeq:  1,
		Value:     model.StringValue("first"),
	}
	second := model.VersionedSample{
		Timestamp: 2,
		WriteSeq:  2,
		Value:     model.StringValue("second"),
	}
	column.appendSample(first)
	if column.count != 1 || len(column.strings) != 1 {
		t.Fatalf("single sample count=%d strings=%d, want one string", column.count, len(column.strings))
	}
	column.reserve(1)
	column.appendSample(second)
	if column.count != 2 {
		t.Fatalf("count = %d, want 2", column.count)
	}
	if column.sampleAt(0).Value.String != "first" || column.sampleAt(1).Value.String != "second" {
		t.Fatalf("samples = %#v/%#v, want first then second", column.sampleAt(0), column.sampleAt(1))
	}

	var boolColumn columnBuffer
	for index := range 130 {
		boolColumn.appendSample(model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     model.BoolValue(index%3 == 0),
		})
	}
	if len(boolColumn.boolBits) != 3 {
		t.Fatalf("bool word count = %d, want 3", len(boolColumn.boolBits))
	}
	if !boolColumn.sampleAt(3).Value.Bool || boolColumn.sampleAt(4).Value.Bool {
		t.Fatalf("bool values around bit boundary are wrong")
	}
}

func TestColumnBufferReservesTypedValueBranches(t *testing.T) {
	intColumn := columnBuffer{fieldType: model.FieldInt64}
	intColumn.reserve(16)
	if cap(intColumn.ints) < 16 {
		t.Fatalf("int cap = %d, want at least 16", cap(intColumn.ints))
	}
	boolColumn := columnBuffer{fieldType: model.FieldBool}
	boolColumn.reserve(130)
	if cap(boolColumn.boolBits) < 3 {
		t.Fatalf("bool word cap = %d, want at least 3", cap(boolColumn.boolBits))
	}
	if boolWords(0) != 0 || boolWords(65) != 2 {
		t.Fatalf("boolWords results = %d/%d, want 0/2", boolWords(0), boolWords(65))
	}
	var unknown columnBuffer
	unknown.fieldType = model.FieldType(99)
	value := unknown.valueAt(0)
	if value.Type != model.FieldType(99) {
		t.Fatalf("unknown value type = %d, want 99", value.Type)
	}
	existingInts := make([]int64, 1, 4)
	if got := growInt64s(existingInts, 2, 1); cap(got) != 4 {
		t.Fatalf("growInt64s no-op cap = %d, want 4", cap(got))
	}
	existingSeqs := make([]uint64, 1, 4)
	if got := growUint64s(existingSeqs, 2, 1); cap(got) != 4 {
		t.Fatalf("growUint64s no-op cap = %d, want 4", cap(got))
	}
	existingFloats := make([]float64, 1, 4)
	if got := growFloat64s(existingFloats, 2, 1); cap(got) != 4 {
		t.Fatalf("growFloat64s no-op cap = %d, want 4", cap(got))
	}
	existingStrings := make([]string, 1, 4)
	if got := growStrings(existingStrings, 2, 1); cap(got) != 4 {
		t.Fatalf("growStrings no-op cap = %d, want 4", cap(got))
	}
	if got := nextSliceCapacity(64, 100, 1); got != 108 {
		t.Fatalf("nextSliceCapacity bounded = %d, want 108", got)
	}
}

func TestColumnBufferMaterializesFilteredOutOfOrderSamples(t *testing.T) {
	var column columnBuffer
	column.appendSample(model.VersionedSample{Timestamp: 20, WriteSeq: 1, Value: model.Int64Value(20)})
	column.appendSample(model.VersionedSample{Timestamp: 10, WriteSeq: 2, Value: model.Int64Value(10)})
	column.appendSample(model.VersionedSample{Timestamp: 10, WriteSeq: 3, Value: model.Int64Value(30)})
	got := compactSamples(&column, Query{Start: 10, End: 10})
	if len(got) != 1 || got[0].Value.Int64 != 30 {
		t.Fatalf("compactSamples() = %#v, want latest timestamp 10 value 30", got)
	}
}

func TestColumnBufferMaterializesSortedRangeWithTightCapacity(t *testing.T) {
	var column columnBuffer
	for index := range 16 {
		column.appendSample(model.VersionedSample{
			Timestamp: int64(index * 10),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		})
	}
	got := compactSamples(&column, Query{Start: 30, End: 50})
	if len(got) != 3 {
		t.Fatalf("compactSamples() len = %d, want 3", len(got))
	}
	if cap(got) != len(got) {
		t.Fatalf("compactSamples() cap = %d, want tight cap %d", cap(got), len(got))
	}
	if got[0].Timestamp != 30 || got[2].Timestamp != 50 {
		t.Fatalf("compactSamples() range = %#v, want timestamps 30..50", got)
	}
	if start, end := sortedRangeBounds(column.times[:column.count], Query{Start: 999, End: 1000}); start != end {
		t.Fatalf("sortedRangeBounds(no overlap) = %d,%d, want equal", start, end)
	}
}

func TestReservationMapPoolReturnsCleanMap(t *testing.T) {
	reservations := borrowReservationMap()
	reservations[columnKey{seriesID: 1, fieldID: 2}] = columnReservation{
		fieldType: model.FieldFloat64,
		count:     1,
	}
	releaseReservationMap(reservations)

	reused := borrowReservationMap()
	if len(reused) != 0 {
		t.Fatalf("borrowReservationMap() len = %d, want 0", len(reused))
	}
	releaseReservationMap(reused)
}

func TestColumnsFromDataSkipsNilColumn(t *testing.T) {
	data := tableData{
		columnKey{seriesID: 1, fieldID: 2}: nil,
	}
	if got := columnsFromData(data, Query{Start: 0, End: 10}); len(got) != 0 {
		t.Fatalf("columnsFromData(nil column) = %d, want 0", len(got))
	}
}

func TestMemTableQueryAvoidsSnapshotClone(t *testing.T) {
	mt := New()
	if err := mt.ApplyBatch([]model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(1, 20, 2, model.Float64Value(2)),
	}); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}

	cloneCalls := 0
	cloneDataHook = func() {
		cloneCalls++
	}
	t.Cleanup(func() {
		cloneDataHook = nil
	})

	columns := mt.Query(Query{Start: 0, End: 30})
	if cloneCalls != 0 {
		t.Fatalf("MemTable.Query cloneData calls = %d, want 0", cloneCalls)
	}
	if len(columns) != 1 || len(columns[0].Samples) != 2 {
		t.Fatalf("Query() columns = %#v, want one column with two samples", columns)
	}
	columns[0].Samples[0].Value = model.Float64Value(99)
	again := mt.Query(Query{Start: 0, End: 30})
	if again[0].Samples[0].Value.Float64 == 99 {
		t.Fatal("MemTable.Query returned internal sample backing array")
	}

	_ = mt.Snapshot()
	if cloneCalls != 1 {
		t.Fatalf("Snapshot cloneData calls = %d, want 1", cloneCalls)
	}
}

func TestReservationMapPoolRejectsInvalidAndLargeEntries(t *testing.T) {
	releaseReservationMap(nil)

	invalid := 1
	reservationMapPool.Put(&invalid)
	got := borrowReservationMap()
	if got == nil {
		t.Fatal("borrowReservationMap() = nil, want map")
	}
	releaseReservationMap(got)

	large := make(map[columnKey]columnReservation, maxPooledReservations+1)
	for index := range maxPooledReservations + 1 {
		large[columnKey{seriesID: uint64(index), fieldID: 1}] = columnReservation{count: 1}
	}
	releaseReservationMap(large)
}

func TestReleaseTableDataRejectsLargeMaps(t *testing.T) {
	releaseTableData(nil)

	large := make(tableData, maxPooledTableColumns+1)
	for index := range maxPooledTableColumns + 1 {
		large[columnKey{seriesID: uint64(index), fieldID: 1}] = nil
	}
	releaseTableData(large)
	if len(large) != 0 {
		t.Fatalf("released large table len = %d, want cleared", len(large))
	}
}

func TestSnapshotReleaseClearsRetainedColumns(t *testing.T) {
	mt := New()
	if err := mt.ApplyBatch([]model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(1, 20, 2, model.Float64Value(2)),
	}); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	snapshot := mt.SnapshotAndReset()
	if snapshot.SampleCount() != 2 {
		t.Fatalf("snapshot sample count = %d, want 2", snapshot.SampleCount())
	}
	snapshot.Release()
	if snapshot.SampleCount() != 0 {
		t.Fatalf("released sample count = %d, want 0", snapshot.SampleCount())
	}
	if got := snapshot.Query(Query{Start: 0, End: 30}); len(got) != 0 {
		t.Fatalf("released query columns = %d, want 0", len(got))
	}
}

func memtableResolvedPoint(seriesID uint64, timestamp int64, seq uint64, value model.FieldValue) model.ResolvedPoint {
	return model.ResolvedPoint{
		SeriesID:  seriesID,
		Timestamp: timestamp,
		WriteSeq:  seq,
		Fields: []model.ResolvedField{
			{
				FieldID:   2,
				FieldName: "usage",
				Type:      value.Type,
				Value:     value,
			},
		},
	}
}
