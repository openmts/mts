package memtable_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
)

func TestMemTableLWWAndSnapshot(t *testing.T) {
	mt := memtable.New()
	older := resolvedPoint(1, 10, 1, model.Float64Value(1))
	newer := resolvedPoint(1, 10, 2, model.Float64Value(3))
	outOfOrder := resolvedPoint(1, 5, 3, model.Float64Value(2))
	stale := resolvedPoint(1, 10, 1, model.Float64Value(9))

	for _, point := range []model.ResolvedPoint{older, newer, outOfOrder, stale} {
		if err := mt.Apply(point); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	}
	if mt.SampleCount() != 4 {
		t.Fatalf("SampleCount() = %d, want 4", mt.SampleCount())
	}

	snap := mt.SnapshotAndReset()
	if mt.SampleCount() != 0 {
		t.Fatalf("SampleCount() after reset = %d, want 0", mt.SampleCount())
	}

	cols := snap.Query(memtable.Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       20,
	})
	if len(cols) != 1 {
		t.Fatalf("column count = %d, want 1", len(cols))
	}
	samples := cols[0].Samples
	if len(samples) != 2 {
		t.Fatalf("sample count = %d, want 2", len(samples))
	}
	if samples[0].Timestamp != 5 {
		t.Fatalf("first timestamp = %d, want 5", samples[0].Timestamp)
	}
	if samples[1].Value.Float64 != 3 {
		t.Fatalf("LWW value = %v, want 3", samples[1].Value.Float64)
	}
}

func TestMemTableMutableQueryAndEmptySnapshot(t *testing.T) {
	mt := memtable.New()
	points := []model.ResolvedPoint{
		{
			SeriesID:  2,
			Timestamp: 2,
			WriteSeq:  1,
			Fields: []model.ResolvedField{
				{FieldID: 3, FieldName: "b", Type: model.FieldBool, Value: model.BoolValue(true)},
			},
		},
		{
			SeriesID:  1,
			Timestamp: 1,
			WriteSeq:  2,
			Fields: []model.ResolvedField{
				{FieldID: 2, FieldName: "a", Type: model.FieldInt64, Value: model.Int64Value(4)},
				{FieldID: 4, FieldName: "c", Type: model.FieldString, Value: model.StringValue("ok")},
			},
		},
	}
	for _, point := range points {
		if err := mt.Apply(point); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	}
	cols := mt.Query(memtable.Query{Start: 0, End: 10})
	if len(cols) != 3 {
		t.Fatalf("column count = %d, want 3", len(cols))
	}
	if cols[0].SeriesID != 1 || cols[0].FieldID != 2 {
		t.Fatalf("first column = series %d field %d, want series 1 field 2", cols[0].SeriesID, cols[0].FieldID)
	}
	if cols[1].SeriesID != 1 || cols[1].FieldID != 4 {
		t.Fatalf("second column = series %d field %d, want series 1 field 4", cols[1].SeriesID, cols[1].FieldID)
	}
	var nilSnapshot *memtable.Snapshot
	if nilSnapshot.SampleCount() != 0 {
		t.Fatal("nil snapshot sample count != 0")
	}
	if got := nilSnapshot.Query(memtable.Query{Start: 10, End: 0}); len(got) != 0 {
		t.Fatalf("nil snapshot query count = %d, want 0", len(got))
	}
	snapshot := mt.Snapshot()
	if snapshot.SampleCount() != 3 {
		t.Fatalf("snapshot sample count = %d, want 3", snapshot.SampleCount())
	}
	if got := snapshot.Query(memtable.Query{Start: 10, End: 0}); len(got) != 0 {
		t.Fatalf("reversed range count = %d, want 0", len(got))
	}
	if got := snapshot.Query(memtable.Query{
		SeriesIDs: map[uint64]struct{}{999: {}},
		FieldIDs:  map[uint32]struct{}{999: {}},
		Start:     0,
		End:       10,
	}); len(got) != 0 {
		t.Fatalf("filtered out count = %d, want 0", len(got))
	}
}

func TestApplyBatchMatchesApply(t *testing.T) {
	points := []model.ResolvedPoint{
		resolvedPoint(1, 10, 1, model.Float64Value(1)),
		resolvedPoint(1, 11, 2, model.Float64Value(2)),
		resolvedPoint(2, 10, 3, model.Int64Value(3)),
	}
	oneByOne := memtable.New()
	for _, point := range points {
		if err := oneByOne.Apply(point); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
	}
	batched := memtable.New()
	if err := batched.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	want := oneByOne.Snapshot().Query(memtable.Query{Start: 0, End: 20})
	got := batched.Snapshot().Query(memtable.Query{Start: 0, End: 20})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyBatch() columns = %#v, want %#v", got, want)
	}
}

func TestMemTableScanColumns(t *testing.T) {
	mt := memtable.New()
	if err := mt.Apply(resolvedPoint(1, 10, 1, model.Float64Value(1))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	stream := mt.ScanColumns(memtable.Query{Start: 0, End: 20})
	if !stream.Next() {
		t.Fatalf("Next() = false, want true err=%v", stream.Err())
	}
	if got := stream.ColumnData(); got.SeriesID != 1 || len(got.Samples) != 1 {
		t.Fatalf("ColumnData() = %#v, want series 1 one sample", got)
	}
	if stream.Next() {
		t.Fatal("Next(after column) = true, want false")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled := mt.ScanColumns(memtable.Query{Context: ctx, Start: 0, End: 20})
	if canceled.Next() {
		t.Fatal("canceled Next() = true, want false")
	}
	if err := canceled.Err(); err == nil {
		t.Fatal("canceled Err() = nil, want context error")
	}
	if err := canceled.Close(); err != nil {
		t.Fatalf("canceled Close() error = %v", err)
	}
}

func TestMemTableApproxMemoryBytesAccountsForColumnBuffersAndStrings(t *testing.T) {
	mt := memtable.New()
	emptyBytes := mt.ApproxMemoryBytes()
	points := []model.ResolvedPoint{
		resolvedPoint(1, 1, 1, model.Float64Value(1)),
		resolvedPoint(1, 2, 2, model.Int64Value(2)),
		resolvedPoint(1, 3, 3, model.BoolValue(true)),
		resolvedPoint(1, 4, 4, model.StringValue("0123456789abcdefghijklmnopqrstuvwxyz")),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	usedBytes := mt.ApproxMemoryBytes()
	if usedBytes <= emptyBytes+int64(len(points)) {
		t.Fatalf("ApproxMemoryBytes() = %d, want greater than empty %d plus sample count", usedBytes, emptyBytes)
	}
	if usedBytes < int64(len("0123456789abcdefghijklmnopqrstuvwxyz")) {
		t.Fatalf("ApproxMemoryBytes() = %d, want at least string payload bytes", usedBytes)
	}
	snapshot := mt.SnapshotAndReset()
	if mt.ApproxMemoryBytes() >= usedBytes {
		t.Fatalf("active ApproxMemoryBytes after reset = %d, want below previous %d", mt.ApproxMemoryBytes(), usedBytes)
	}
	if snapshot.ApproxMemoryBytes() < usedBytes {
		t.Fatalf("snapshot ApproxMemoryBytes = %d, want at least previous active %d", snapshot.ApproxMemoryBytes(), usedBytes)
	}
	snapshot.Release()
	if snapshot.ApproxMemoryBytes() != 0 {
		t.Fatalf("released snapshot ApproxMemoryBytes = %d, want 0", snapshot.ApproxMemoryBytes())
	}
}

func TestAppendBufferKeepsLatestWriteSeq(t *testing.T) {
	mt := memtable.New()
	points := []model.ResolvedPoint{
		resolvedPoint(1, 10, 1, model.Float64Value(1)),
		resolvedPoint(1, 10, 3, model.Float64Value(3)),
		resolvedPoint(1, 10, 2, model.Float64Value(2)),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	got := mt.Query(memtable.Query{Start: 0, End: 20})
	if len(got) != 1 || len(got[0].Samples) != 1 {
		t.Fatalf("Query() = %#v, want one compacted sample", got)
	}
	if got[0].Samples[0].Value.Float64 != 3 {
		t.Fatalf("latest value = %v, want 3", got[0].Samples[0].Value.Float64)
	}
}

func TestSnapshotColumnsAreSortedAndCompacted(t *testing.T) {
	mt := memtable.New()
	points := []model.ResolvedPoint{
		resolvedPoint(2, 20, 1, model.Int64Value(20)),
		resolvedPoint(1, 10, 1, model.Float64Value(1)),
		resolvedPoint(1, 10, 2, model.Float64Value(2)),
		resolvedPoint(1, 5, 3, model.Float64Value(5)),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	got := mt.Snapshot().Columns(memtable.Query{Start: 0, End: 30})
	if len(got) != 2 {
		t.Fatalf("Columns() len = %d, want 2", len(got))
	}
	if got[0].SeriesID != 1 || got[1].SeriesID != 2 {
		t.Fatalf("Columns() order = %#v", got)
	}
	samples := got[0].Samples
	if len(samples) != 2 || samples[0].Timestamp != 5 || samples[1].Timestamp != 10 {
		t.Fatalf("Samples order = %#v, want timestamps 5,10", samples)
	}
	if samples[1].Value.Float64 != 2 {
		t.Fatalf("LWW value = %v, want 2", samples[1].Value.Float64)
	}
}

func TestSnapshotForEachSeriesStreamsSortedSeries(t *testing.T) {
	mt := memtable.New()
	points := []model.ResolvedPoint{
		resolvedPointWithFields(2, 20, 1, []model.ResolvedField{
			{FieldID: 3, Type: model.FieldInt64, Value: model.Int64Value(20)},
		}),
		resolvedPointWithFields(1, 10, 1, []model.ResolvedField{
			{FieldID: 4, Type: model.FieldBool, Value: model.BoolValue(true)},
			{FieldID: 2, Type: model.FieldFloat64, Value: model.Float64Value(1)},
		}),
		resolvedPointWithFields(1, 10, 2, []model.ResolvedField{
			{FieldID: 2, Type: model.FieldFloat64, Value: model.Float64Value(2)},
		}),
	}
	if err := mt.ApplyBatch(points); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}

	var gotSeries []uint64
	var gotFields [][]uint32
	err := mt.Snapshot().ForEachSeries(memtable.Query{Start: 0, End: 30}, func(seriesID uint64, columns []model.ColumnData) error {
		gotSeries = append(gotSeries, seriesID)
		fields := make([]uint32, 0, len(columns))
		for _, column := range columns {
			fields = append(fields, column.FieldID)
			if len(column.Samples) == 0 {
				t.Fatalf("series %d field %d has empty samples", seriesID, column.FieldID)
			}
		}
		gotFields = append(gotFields, fields)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachSeries() error = %v", err)
	}
	if !reflect.DeepEqual(gotSeries, []uint64{1, 2}) {
		t.Fatalf("series order = %v, want [1 2]", gotSeries)
	}
	if !reflect.DeepEqual(gotFields, [][]uint32{{2, 4}, {3}}) {
		t.Fatalf("field groups = %v, want [[2 4] [3]]", gotFields)
	}
}

func TestSnapshotAndResetKeepsSnapshotStable(t *testing.T) {
	mt := memtable.New()
	if err := mt.Apply(resolvedPoint(1, 10, 1, model.Float64Value(1))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	snapshot := mt.SnapshotAndReset()
	if mt.SampleCount() != 0 {
		t.Fatalf("SampleCount() after reset = %d, want 0", mt.SampleCount())
	}
	if err := mt.Apply(resolvedPoint(1, 10, 2, model.Float64Value(2))); err != nil {
		t.Fatalf("Apply() second error = %v", err)
	}
	got := snapshot.Query(memtable.Query{Start: 0, End: 20})
	if len(got) != 1 || len(got[0].Samples) != 1 {
		t.Fatalf("old snapshot columns = %#v, want one sample", got)
	}
	if got[0].Samples[0].Value.Float64 != 1 {
		t.Fatalf("old snapshot value = %v, want 1", got[0].Samples[0].Value.Float64)
	}
}

func TestRestoreMergesSnapshotWithCurrentData(t *testing.T) {
	mt := memtable.New()
	if err := mt.Apply(resolvedPoint(1, 10, 1, model.Float64Value(1))); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	snapshot := mt.SnapshotAndReset()
	if err := mt.Apply(resolvedPoint(1, 10, 2, model.Float64Value(2))); err != nil {
		t.Fatalf("Apply(newer) error = %v", err)
	}
	if err := mt.Apply(resolvedPoint(2, 20, 3, model.Int64Value(3))); err != nil {
		t.Fatalf("Apply(other) error = %v", err)
	}
	mt.Restore(snapshot)
	if mt.SampleCount() != 3 {
		t.Fatalf("SampleCount() = %d, want 3", mt.SampleCount())
	}
	got := mt.Query(memtable.Query{Start: 0, End: 30})
	if len(got) != 2 {
		t.Fatalf("column count = %d, want 2", len(got))
	}
	if got[0].Samples[0].Value.Float64 != 2 {
		t.Fatalf("restored stale value replaced newer value: %#v", got[0].Samples[0].Value)
	}
	mt.Restore(nil)
	if mt.SampleCount() != 3 {
		t.Fatalf("SampleCount() after nil restore = %d, want 3", mt.SampleCount())
	}
}

func resolvedPoint(seriesID uint64, timestamp int64, seq uint64, value model.FieldValue) model.ResolvedPoint {
	return resolvedPointWithFields(seriesID, timestamp, seq, []model.ResolvedField{
		{
			FieldID:   2,
			FieldName: "usage",
			Type:      value.Type,
			Value:     value,
		},
	})
}

func resolvedPointWithFields(
	seriesID uint64,
	timestamp int64,
	seq uint64,
	fields []model.ResolvedField,
) model.ResolvedPoint {
	return model.ResolvedPoint{
		SeriesID:  seriesID,
		Timestamp: timestamp,
		WriteSeq:  seq,
		Fields:    fields,
	}
}
