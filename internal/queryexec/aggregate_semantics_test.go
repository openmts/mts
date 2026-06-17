package queryexec

import (
	"testing"
	"time"

	"codeberg.org/mts/mts/internal/model"
)

func TestFirstLastAggregatesUseBoundaryTimestamps(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:   1,
			FieldName:  "value",
			FieldType:  model.FieldInt64,
			Timestamps: []int64{30, 10, 20},
			Values: []model.FieldValue{
				model.Int64Value(3),
				model.Int64Value(1),
				model.Int64Value(2),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "value", Function: "first"},
			{Field: "value", Function: "last"},
		},
	)

	if !stream.Next() {
		t.Fatalf("Next(first) = false err=%v", stream.Err())
	}
	first := stream.Column()
	if first.Timestamps[0] != 10 || first.Values[0].Int64 != 1 {
		t.Fatalf("first aggregate = %#v, want timestamp 10 value 1", first)
	}
	if !stream.Next() {
		t.Fatalf("Next(last) = false err=%v", stream.Err())
	}
	last := stream.Column()
	if last.Timestamps[0] != 30 || last.Values[0].Int64 != 3 {
		t.Fatalf("last aggregate = %#v, want timestamp 30 value 3", last)
	}
}

func TestAggregateWindowMergesAcrossShardAndPartBoundaries(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:  1,
			FieldName: "value",
			FieldType: model.FieldFloat64,
			Timestamps: []int64{
				int64(1500 * time.Millisecond),
				int64(500 * time.Millisecond),
				int64(2500 * time.Millisecond),
			},
			Values: []model.FieldValue{
				model.Float64Value(2),
				model.Float64Value(1),
				model.Float64Value(3),
			},
		}}),
		[]model.AggregateSpec{{Field: "value", Function: "sum"}},
		time.Second,
	)

	if !stream.Next() {
		t.Fatalf("Next(sum) = false err=%v", stream.Err())
	}
	column := stream.Column()
	wantTimes := []int64{0, int64(time.Second), int64(2 * time.Second)}
	wantValues := []float64{1, 2, 3}
	if len(column.Timestamps) != len(wantTimes) {
		t.Fatalf("window count = %d, want %d column=%#v", len(column.Timestamps), len(wantTimes), column)
	}
	for index := range wantTimes {
		if column.Timestamps[index] != wantTimes[index] || column.Values[index].Float64 != wantValues[index] {
			t.Fatalf("window %d = (%d,%v), want (%d,%v)", index, column.Timestamps[index], column.Values[index].Float64, wantTimes[index], wantValues[index])
		}
	}
}
