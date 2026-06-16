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
	wantSampleCapacity := len(points)
	if cap(column.samples) != wantSampleCapacity {
		t.Fatalf("column sample cap = %d, want %d", cap(column.samples), wantSampleCapacity)
	}
	if len(column.samples) != len(points) {
		t.Fatalf("dense sample len = %d, want %d", len(column.samples), len(points))
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
