package queryexec

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestPaginatedColumnStreamAppliesOffsetAndLimit(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:   1,
		FieldName:  "value",
		Timestamps: []int64{1, 2, 3, 4},
		Values: []model.FieldValue{
			model.Int64Value(1),
			model.Int64Value(2),
			model.Int64Value(3),
			model.Int64Value(4),
		},
	}})
	stream := NewPaginatedColumnStream(source, 2, 1)
	if !stream.Next() {
		t.Fatal("Next() = false, want true")
	}
	column := stream.Column()
	if len(column.Values) != 2 || column.Values[0].Int64 != 2 || column.Values[1].Int64 != 3 {
		t.Fatalf("paged values = %#v, want 2 and 3", column.Values)
	}
	if stream.Next() {
		t.Fatal("Next(after limit) = true, want false")
	}
}

func TestPaginatedRowStreamAppliesOffsetAndLimit(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{Timestamp: 1},
		{Timestamp: 2},
		{Timestamp: 3},
		{Timestamp: 4},
	})
	stream := NewPaginatedRowStream(source, 2, 1)
	var timestamps []int64
	for stream.Next() {
		timestamps = append(timestamps, stream.Row().Timestamp)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if len(timestamps) != 2 || timestamps[0] != 2 || timestamps[1] != 3 {
		t.Fatalf("timestamps = %v, want [2 3]", timestamps)
	}
}

func TestBudgetRowStreamStopsWhenMaxSamplesExceeded(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{Timestamp: 1},
		{Timestamp: 2},
		{Timestamp: 3},
	})
	stream := NewBudgetRowStream(source, model.QueryBudget{MaxSamples: 2})
	if !stream.Next() {
		t.Fatalf("Next(1) = false err=%v", stream.Err())
	}
	if !stream.Next() {
		t.Fatalf("Next(2) = false err=%v", stream.Err())
	}
	if stream.Next() {
		t.Fatal("Next(3) = true, want false")
	}
	if !errors.Is(stream.Err(), ErrReadBudgetExceeded) {
		t.Fatalf("Err() = %v, want ErrReadBudgetExceeded", stream.Err())
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBudgetColumnStreamReturnsReadBudgetError(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:   1,
		FieldName:  "value",
		Timestamps: []int64{1, 2},
		Values: []model.FieldValue{
			model.Int64Value(1),
			model.Int64Value(2),
		},
	}})
	stream := NewBudgetColumnStream(source, model.QueryBudget{MaxSamples: 1})
	if stream.Next() {
		t.Fatal("Next() = true, want false")
	}
	if !errors.Is(stream.Err(), ErrReadBudgetExceeded) {
		t.Fatalf("Err() = %v, want ErrReadBudgetExceeded", stream.Err())
	}
	if stream.Err().Error() == "" {
		t.Fatal("Err().Error() is empty")
	}
	if got := stream.Column(); got.SeriesID != 0 {
		t.Fatalf("Column() after budget error = %#v, want zero", got)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPaginatedColumnStreamCloseAndErr(t *testing.T) {
	stream := NewPaginatedColumnStream(NewSliceColumnSeriesStream(nil), 1, 0)
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if stream.Next() {
		t.Fatal("Next(after close) = true, want false")
	}
}

func TestBudgetColumnStreamAllowsWithinLimit(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:   2,
		FieldName:  "value",
		Timestamps: []int64{1},
		Values:     []model.FieldValue{model.Int64Value(1)},
	}})
	stream := NewBudgetColumnStream(source, model.QueryBudget{MaxSamples: 2})
	if !stream.Next() {
		t.Fatalf("Next() = false, want true err=%v", stream.Err())
	}
	if got := stream.Column(); got.SeriesID != 2 {
		t.Fatalf("Column() = %#v, want series 2", got)
	}
	if stream.Next() {
		t.Fatal("Next(after source end) = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
