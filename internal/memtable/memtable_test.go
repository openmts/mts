package memtable_test

import (
	"testing"

	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
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
	if mt.SampleCount() != 2 {
		t.Fatalf("SampleCount() = %d, want 2", mt.SampleCount())
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

func resolvedPoint(seriesID uint64, timestamp int64, seq uint64, value model.FieldValue) model.ResolvedPoint {
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
