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

func TestApplyBatchSkipsReservationMapForMostlyUniqueLargeBatch(t *testing.T) {
	mt := New()
	points := make([]model.ResolvedPoint, 256)
	for index := range points {
		points[index] = memtableResolvedPoint(uint64(index+1), int64(index), uint64(index+1), model.Float64Value(1))
	}
	borrows := 0
	borrowReservationMapHook = func() {
		borrows++
	}
	t.Cleanup(func() {
		borrowReservationMapHook = nil
	})

	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if borrows != 0 {
		t.Fatalf("reservation map borrows = %d, want 0 for mostly unique large batch", borrows)
	}
	if mt.SampleCount() != len(points) {
		t.Fatalf("sample count = %d, want %d", mt.SampleCount(), len(points))
	}
}

func TestApplyTypedBatchWritesColumnValues(t *testing.T) {
	mt := New()
	batch := model.ResolvedTypedBatch{
		Measurement: "cpu",
		Timestamps:  []int64{10, 20},
		SeriesIDs:   []uint64{1, 1},
		WriteSeqs:   []uint64{3, 4},
		Fields: []model.ResolvedTypedFieldColumn{
			{
				FieldID:       1,
				Name:          "usage",
				Type:          model.FieldFloat64,
				Float64Values: []float64{1.5, 2.5},
			},
			{
				FieldID:    2,
				Name:       "active",
				Type:       model.FieldBool,
				BoolValues: []bool{true, false},
			},
		},
	}

	if err := mt.ApplyTypedBatch(batch, nil); err != nil {
		t.Fatalf("ApplyTypedBatch() error = %v", err)
	}
	columns := mt.Query(Query{Start: 0, End: 30})
	if len(columns) != 2 {
		t.Fatalf("columns len = %d, want 2: %#v", len(columns), columns)
	}
	if columns[0].FieldID != 1 || columns[0].Samples[1].Value.Float64 != 2.5 {
		t.Fatalf("float column = %#v, want field 1 second value 2.5", columns[0])
	}
	if columns[1].FieldID != 2 || columns[1].Samples[0].Value.Bool != true || columns[1].Samples[1].Value.Bool != false {
		t.Fatalf("bool column = %#v, want true/false values", columns[1])
	}
}

func TestApplyBatchKeepsReservationMapForRepeatedSeries(t *testing.T) {
	mt := New()
	points := make([]model.ResolvedPoint, 16)
	for index := range points {
		points[index] = memtableResolvedPoint(1, int64(index), uint64(index+1), model.Float64Value(1))
	}
	borrows := 0
	borrowReservationMapHook = func() {
		borrows++
	}
	t.Cleanup(func() {
		borrowReservationMapHook = nil
	})

	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if borrows == 0 {
		t.Fatal("reservation map was not borrowed for repeated-series batch")
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

func TestColumnBufferAppendUsesBoundedCapacityGrowth(t *testing.T) {
	var column columnBuffer
	column.fieldType = model.FieldFloat64
	for index := range 1024 {
		column.appendSample(model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		})
	}
	if extra := cap(column.times) - column.count; extra > 8 {
		t.Fatalf("time capacity extra = %d, want <= 8", extra)
	}
	if extra := cap(column.writeSeqs) - column.count; extra > 8 {
		t.Fatalf("writeSeq capacity extra = %d, want <= 8", extra)
	}
	if extra := cap(column.floats) - column.count; extra > 8 {
		t.Fatalf("float capacity extra = %d, want <= 8", extra)
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

func TestHasUniqueSeriesPrefixUsesLinearScan(t *testing.T) {
	unique := make([]model.ResolvedPoint, uniqueSeriesReservationBypassMinPoints)
	for index := range unique {
		unique[index] = memtableResolvedPoint(uint64(index+1), int64(index+1), uint64(index+1), model.Float64Value(1))
	}
	if !hasUniqueSeriesPrefix(unique, len(unique)) {
		t.Fatal("hasUniqueSeriesPrefix(unique) = false, want true")
	}
	dup := append([]model.ResolvedPoint{}, unique...)
	dup[len(dup)-1].SeriesID = dup[0].SeriesID
	if hasUniqueSeriesPrefix(dup, len(dup)) {
		t.Fatal("hasUniqueSeriesPrefix(duplicate) = true, want false")
	}
}

func TestHasUniqueTypedSeriesPrefixUsesLinearScan(t *testing.T) {
	count := uniqueSeriesReservationBypassMinPoints
	batch := model.ResolvedTypedBatch{
		Timestamps: make([]int64, count),
		SeriesIDs:  make([]uint64, count),
		WriteSeqs:  make([]uint64, count),
		Fields: []model.ResolvedTypedFieldColumn{{
			FieldID:       1,
			Name:          "v",
			Type:          model.FieldFloat64,
			Float64Values: make([]float64, count),
		}},
	}
	for index := range count {
		batch.Timestamps[index] = int64(index + 1)
		batch.SeriesIDs[index] = uint64(index + 1)
		batch.WriteSeqs[index] = uint64(index + 1)
		batch.Fields[0].Float64Values[index] = float64(index)
	}
	if !hasUniqueTypedSeriesPrefix(batch, nil, count) {
		t.Fatal("hasUniqueTypedSeriesPrefix(unique) = false, want true")
	}
	batch.SeriesIDs[count-1] = batch.SeriesIDs[0]
	if hasUniqueTypedSeriesPrefix(batch, nil, count) {
		t.Fatal("hasUniqueTypedSeriesPrefix(duplicate) = true, want false")
	}
}

func TestColumnBufferAppendSampleUsesReservedCapacity(t *testing.T) {
	column := borrowColumnBuffer(1, 1, model.FieldFloat64)
	if delta := column.reserve(8); delta <= 0 {
		t.Fatalf("reserve(8) delta = %d, want positive", delta)
	}
	beforeTimesCap := cap(column.times)
	beforeFloatsCap := cap(column.floats)
	beforeMem := column.memBytes
	for index := 0; index < 8; index++ {
		delta := column.appendSample(model.VersionedSample{
			Timestamp: int64(index + 1),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		})
		if delta != 0 {
			t.Fatalf("appendSample(%d) delta = %d, want 0 after reserve", index, delta)
		}
	}
	if cap(column.times) != beforeTimesCap || cap(column.floats) != beforeFloatsCap {
		t.Fatalf("capacity changed after reserved append: times %d->%d floats %d->%d",
			beforeTimesCap, cap(column.times), beforeFloatsCap, cap(column.floats))
	}
	if column.memBytes != beforeMem {
		t.Fatalf("memBytes = %d, want unchanged reserved capacity %d", column.memBytes, beforeMem)
	}
	if column.count != 8 {
		t.Fatalf("count = %d, want 8", column.count)
	}
	releaseColumnBuffer(column)
}

func TestMemTableApproxMemoryBytesAvoidsFullScan(t *testing.T) {
	mt := New()
	if err := mt.ApplyBatch([]model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(2, 20, 2, model.StringValue("payload")),
		memtableResolvedPoint(3, 30, 3, model.BoolValue(true)),
	}); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}

	scans := 0
	approxDataHook = func() {
		scans++
	}
	t.Cleanup(func() {
		approxDataHook = nil
	})

	if got := mt.ApproxMemoryBytes(); got <= 0 {
		t.Fatalf("ApproxMemoryBytes() = %d, want positive", got)
	}
	if scans != 0 {
		t.Fatalf("ApproxMemoryBytes() full scans = %d, want 0", scans)
	}

	snapshot := mt.SnapshotAndReset()
	if got := snapshot.ApproxMemoryBytes(); got <= 0 {
		t.Fatalf("snapshot ApproxMemoryBytes() = %d, want positive", got)
	}
	if scans != 0 {
		t.Fatalf("snapshot ApproxMemoryBytes() full scans = %d, want 0", scans)
	}
}

func TestMemTableTrackedMemoryMatchesFullCalculation(t *testing.T) {
	mt := New()
	points := []model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(1, 20, 2, model.Float64Value(2)),
		memtableResolvedPoint(2, 30, 3, model.StringValue("payload-a")),
		memtableResolvedPoint(2, 40, 4, model.StringValue("payload-b")),
		memtableResolvedPoint(3, 50, 5, model.BoolValue(true)),
		memtableResolvedPoint(3, 60, 6, model.BoolValue(false)),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if got, want := mt.ApproxMemoryBytes(), approxTableDataBytes(mt.data); got != want {
		t.Fatalf("ApproxMemoryBytes() = %d, want full calculation %d", got, want)
	}
	cloned := mt.Snapshot()
	if got, want := cloned.ApproxMemoryBytes(), approxTableDataBytes(cloned.data); got != want {
		t.Fatalf("cloned snapshot ApproxMemoryBytes() = %d, want full calculation %d", got, want)
	}

	snapshot := mt.SnapshotAndReset()
	if got, want := snapshot.ApproxMemoryBytes(), approxTableDataBytes(snapshot.data); got != want {
		t.Fatalf("snapshot ApproxMemoryBytes() = %d, want full calculation %d", got, want)
	}
	if err := mt.Apply(memtableResolvedPoint(4, 70, 7, model.Int64Value(7))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	mt.Restore(snapshot)
	if got, want := mt.ApproxMemoryBytes(), approxTableDataBytes(mt.data); got != want {
		t.Fatalf("restored ApproxMemoryBytes() = %d, want full calculation %d", got, want)
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

func TestReleaseTableDataRetainsReusableWideMaps(t *testing.T) {
	resetTableDataPoolForTest()
	t.Cleanup(resetTableDataPoolForTest)
	data := make(tableData, maxPooledTableColumns)
	releaseTableData(data)
	if got := tableDataPoolLenForTest(); got != 1 {
		t.Fatalf("table data pool len = %d, want 1", got)
	}
	_ = borrowTableData()
	if got := tableDataPoolLenForTest(); got != 0 {
		t.Fatalf("table data pool len after borrow = %d, want 0", got)
	}
}

func TestColumnKeyFreeListBoundsRetainedSlices(t *testing.T) {
	resetColumnKeyPoolForTest()
	t.Cleanup(resetColumnKeyPoolForTest)
	releaseColumnKeys(nil)
	releaseColumnKeys(make([]columnKey, maxPooledColumnKeys+1))
	if got := columnKeyPoolLenForTest(); got != 0 {
		t.Fatalf("column key pool len after nil/large release = %d, want 0", got)
	}
	keys := make([]columnKey, maxPooledColumnKeys)
	releaseColumnKeys(keys)
	if got := columnKeyPoolLenForTest(); got != 1 {
		t.Fatalf("column key pool len = %d, want 1", got)
	}
	reused := borrowColumnKeys(maxPooledColumnKeys)
	if cap(reused) < maxPooledColumnKeys {
		t.Fatalf("reused cap = %d, want at least %d", cap(reused), maxPooledColumnKeys)
	}
	if got := columnKeyPoolLenForTest(); got != 0 {
		t.Fatalf("column key pool len after borrow = %d, want 0", got)
	}
}

func TestColumnRetainsLargeBackingChecksAllTypedSlices(t *testing.T) {
	cases := []struct {
		name   string
		column columnBuffer
	}{
		{name: "writeSeqs", column: columnBuffer{writeSeqs: make([]uint64, 0, maxPooledColumnCapacity+1)}},
		{name: "floats", column: columnBuffer{floats: make([]float64, 0, maxPooledColumnCapacity+1)}},
		{name: "ints", column: columnBuffer{ints: make([]int64, 0, maxPooledColumnCapacity+1)}},
		{name: "strings", column: columnBuffer{strings: make([]string, 0, maxPooledColumnCapacity+1)}},
		{name: "bools", column: columnBuffer{boolBits: make([]uint64, 0, boolWords(maxPooledColumnCapacity)+1)}},
		{name: "small", column: columnBuffer{times: make([]int64, 0, maxPooledColumnCapacity)}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := columnRetainsLargeBacking(&tt.column)
			want := tt.name != "small"
			if got != want {
				t.Fatalf("columnRetainsLargeBacking() = %v, want %v", got, want)
			}
		})
	}
}

func TestSnapshotReleaseAllowsColumnBufferReuse(t *testing.T) {
	mt := New()
	if err := mt.Apply(memtableResolvedPoint(1, 10, 1, model.Float64Value(1))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	reused := 0
	borrowColumnBufferHook = func(fromPool bool) {
		if fromPool {
			reused++
		}
	}
	t.Cleanup(func() {
		borrowColumnBufferHook = nil
	})

	mt.SnapshotAndReset().Release()
	if err := mt.Apply(memtableResolvedPoint(2, 20, 2, model.Float64Value(2))); err != nil {
		t.Fatalf("Apply(reuse) error = %v", err)
	}
	if reused == 0 {
		t.Fatal("column buffer was not reused after snapshot release")
	}
}

func TestReleaseColumnBufferDropsLargeBackingArrays(t *testing.T) {
	column := &columnBuffer{fieldType: model.FieldFloat64}
	column.reserve(maxPooledColumnCapacity + 1)
	releaseColumnBuffer(column)

	reused := borrowColumnBuffer(1, 2, model.FieldFloat64)
	if cap(reused.times) > maxPooledColumnCapacity {
		t.Fatalf("reused time cap = %d, want <= %d", cap(reused.times), maxPooledColumnCapacity)
	}
	releaseColumnBuffer(reused)
}

func TestColumnBufferFreeListBoundsRetainedObjects(t *testing.T) {
	resetColumnBufferPoolForTest()
	t.Cleanup(resetColumnBufferPoolForTest)
	for range maxPooledColumnBuffers + 1 {
		releaseColumnBuffer(&columnBuffer{})
	}
	if got := columnBufferPoolLenForTest(); got != maxPooledColumnBuffers {
		t.Fatalf("column buffer pool len = %d, want %d", got, maxPooledColumnBuffers)
	}
	for range maxPooledColumnBuffers {
		_ = borrowColumnBuffer(1, 1, model.FieldFloat64)
	}
	if got := columnBufferPoolLenForTest(); got != 0 {
		t.Fatalf("column buffer pool len after borrow = %d, want 0", got)
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

func TestStatsSnapshotReportsSeriesFieldsColumnsAndNilSnapshot(t *testing.T) {
	mt := New()
	if err := mt.ApplyBatch([]model.ResolvedPoint{
		memtableResolvedPoint(1, 10, 1, model.Float64Value(1)),
		memtableResolvedPoint(1, 20, 2, model.Int64Value(2)),
		memtableResolvedPoint(2, 30, 3, model.Float64Value(3)),
	}); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	stats := mt.StatsSnapshot()
	if stats.Samples != 3 || stats.Series != 2 || stats.Fields != 1 || stats.Columns != 2 || stats.Bytes == 0 {
		t.Fatalf("StatsSnapshot() = %#v, want samples/series/fields/columns/bytes", stats)
	}
	snapshot := mt.Snapshot()
	snapshotStats := snapshot.StatsSnapshot()
	if snapshotStats.Samples != stats.Samples || snapshotStats.Columns != stats.Columns {
		t.Fatalf("Snapshot stats = %#v, want match %#v", snapshotStats, stats)
	}
	var nilSnapshot *Snapshot
	if nilSnapshot.StatsSnapshot() != (Stats{}) {
		t.Fatalf("nil snapshot stats = %#v, want zero", nilSnapshot.StatsSnapshot())
	}
	if nilSnapshot.SampleCount() != 0 || nilSnapshot.ApproxMemoryBytes() != 0 {
		t.Fatalf("nil snapshot counters = %d/%d, want zero", nilSnapshot.SampleCount(), nilSnapshot.ApproxMemoryBytes())
	}
	snapshot.Release()
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
