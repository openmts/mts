package queryexec

import (
	"context"
	"errors"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestSliceColumnStreamDecoratesOnlyWhenColumnIsRead(t *testing.T) {
	decorations := 0
	stream := NewSliceColumnStream(
		[]model.ColumnData{
			{
				SeriesID:  10,
				FieldID:   20,
				FieldType: model.FieldFloat64,
				Samples: []model.VersionedSample{
					{Timestamp: 1, Value: model.Float64Value(11)},
				},
			},
		},
		func(column model.ColumnData) (model.ColumnSeries, bool) {
			decorations++
			return model.ColumnSeries{
				SeriesID:   column.SeriesID,
				FieldID:    column.FieldID,
				FieldType:  column.FieldType,
				Timestamps: []int64{column.Samples[0].Timestamp},
				Values:     []model.FieldValue{column.Samples[0].Value},
			}, true
		},
	)

	if decorations != 0 {
		t.Fatalf("decorations after constructor = %d, want 0", decorations)
	}
	if got := stream.Column(); got.SeriesID != 0 {
		t.Fatalf("Column(before Next) = %#v, want zero", got)
	}
	if !stream.Next() {
		t.Fatal("Next() = false, want true")
	}
	if decorations != 0 {
		t.Fatalf("decorations after Next = %d, want 0", decorations)
	}
	if got := stream.Column(); got.SeriesID != 10 || got.FieldID != 20 {
		t.Fatalf("Column() = %#v, want series 10 field 20", got)
	}
	if decorations != 1 {
		t.Fatalf("decorations after Column = %d, want 1", decorations)
	}
	if got := stream.Column(); got.SeriesID != 10 {
		t.Fatalf("Column(second read) = %#v, want cached series 10", got)
	}
	if decorations != 1 {
		t.Fatalf("decorations after second Column = %d, want 1", decorations)
	}
	if stream.Next() {
		t.Fatal("Next(after end) = true, want false")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if stream.Next() {
		t.Fatal("Next(after close) = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestSliceRowStreamBoundsAndClose(t *testing.T) {
	stream := NewSliceRowStream([]model.Row{{SeriesID: 3}})
	if got := stream.Row(); got.SeriesID != 0 {
		t.Fatalf("Row(before Next) = %#v, want zero", got)
	}
	if !stream.Next() {
		t.Fatal("Next() = false, want true")
	}
	if got := stream.Row(); got.SeriesID != 3 {
		t.Fatalf("Row() = %#v, want series 3", got)
	}
	if stream.Next() {
		t.Fatal("Next(after last row) = true, want false")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if stream.Next() {
		t.Fatal("Next(after close) = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestErrorAndContextColumnDataStreams(t *testing.T) {
	stopErr := errors.New("stop")
	errStream := NewErrorColumnDataStream(stopErr)
	if errStream.Next() {
		t.Fatal("error stream Next() = true, want false")
	}
	if got := errStream.ColumnData(); got.SeriesID != 0 {
		t.Fatalf("error stream ColumnData() = %#v, want zero", got)
	}
	if !errors.Is(errStream.Err(), stopErr) {
		t.Fatalf("error stream Err() = %v, want %v", errStream.Err(), stopErr)
	}
	if err := errStream.Close(); err != nil {
		t.Fatalf("error stream Close() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	contextStream := WithContextColumnDataStream(ctx, NewSliceColumnDataStream([]model.ColumnData{{SeriesID: 1}}))
	if contextStream.Next() {
		t.Fatal("context stream Next() = true, want false")
	}
	if !errors.Is(contextStream.Err(), context.Canceled) {
		t.Fatalf("context stream Err() = %v, want context.Canceled", contextStream.Err())
	}
	if got := contextStream.ColumnData(); got.SeriesID != 0 {
		t.Fatalf("context stream ColumnData() = %#v, want zero", got)
	}
	if err := contextStream.Close(); err != nil {
		t.Fatalf("context stream Close() error = %v", err)
	}

	plain := NewSliceColumnDataStream([]model.ColumnData{{SeriesID: 2}})
	var nilContext context.Context
	if got := WithContextColumnDataStream(nilContext, plain); got != plain {
		t.Fatal("WithContextColumnDataStream(nil) did not return original stream")
	}

	decorated := NewDecoratedColumnStream(nil, nil)
	if decorated.Next() {
		t.Fatal("nil decorated Next() = true, want false")
	}
	if got := decorated.Column(); got.SeriesID != 0 {
		t.Fatalf("nil decorated Column() = %#v, want zero", got)
	}
	if err := decorated.Err(); err != nil {
		t.Fatalf("nil decorated Err() = %v, want nil", err)
	}
	if err := decorated.Close(); err != nil {
		t.Fatalf("nil decorated Close() = %v, want nil", err)
	}

	wrappedNil := WithContextColumnDataStream(context.Background(), nil)
	if wrappedNil.Next() {
		t.Fatal("nil context stream Next() = true, want false")
	}
	if got := wrappedNil.ColumnData(); got.SeriesID != 0 {
		t.Fatalf("nil context stream ColumnData() = %#v, want zero", got)
	}
	if err := wrappedNil.Err(); err != nil {
		t.Fatalf("nil context stream Err() = %v, want nil", err)
	}
	if err := wrappedNil.Close(); err != nil {
		t.Fatalf("nil context stream Close() = %v, want nil", err)
	}
}
