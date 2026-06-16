package memtable

import (
	"testing"

	"codeberg.org/mts/mts/internal/model"
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
	cloned.samples[0].Timestamp = 99
	if dst.samples[0].Timestamp == 99 {
		t.Fatal("cloneColumn() shared sample backing array")
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
	if cap(column.samples) < len(points) {
		t.Fatalf("column sample cap = %d, want at least %d", cap(column.samples), len(points))
	}
	if len(column.samples) != len(points) {
		t.Fatalf("dense sample len = %d, want %d", len(column.samples), len(points))
	}
}

func TestColumnBufferReserveGrowsForSmallIncrementalBatches(t *testing.T) {
	var column columnBuffer
	column.reserve(8)
	firstCapacity := cap(column.samples)
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
	if cap(column.samples) <= firstCapacity {
		t.Fatalf("grown capacity = %d, want greater than %d", cap(column.samples), firstCapacity)
	}
	if cap(column.samples) > firstCapacity+3 {
		t.Fatalf("grown capacity = %d, want bounded slack over %d", cap(column.samples), firstCapacity)
	}
}

func TestColumnBufferMigratesInlineFirstSampleToDenseSlice(t *testing.T) {
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
	if column.count != 1 || len(column.samples) != 0 {
		t.Fatalf("single sample count=%d len=%d, want inline only", column.count, len(column.samples))
	}
	column.reserve(1)
	column.appendSample(second)
	if column.count != 2 {
		t.Fatalf("count = %d, want 2", column.count)
	}
	if len(column.samples) != 2 {
		t.Fatalf("dense sample len = %d, want 2", len(column.samples))
	}
	if column.samples[0].Value.String != "first" || column.samples[1].Value.String != "second" {
		t.Fatalf("dense samples = %#v, want first then second", column.samples)
	}
	if column.first != (model.VersionedSample{}) {
		t.Fatalf("first sample was not cleared: %#v", column.first)
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
