package queryexec

import (
	"context"
	"errors"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestContextColumnStreamStopsAndClosesInnerOnDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := &recordingColumnStream{
		columns: []model.ColumnSeries{{
			SeriesID:   1,
			FieldName:  "value",
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.Int64Value(1)},
		}},
	}
	stream := WithContextColumnStream(ctx, inner)

	cancel()
	if stream.Next() {
		t.Fatal("Next() = true, want false after context cancellation")
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("Err() = %v, want context.Canceled", stream.Err())
	}
	if inner.nextCalls != 0 {
		t.Fatalf("inner Next calls = %d, want 0", inner.nextCalls)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("inner Close calls after cancel = %d, want 1", inner.closeCalls)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("inner Close calls = %d, want 1", inner.closeCalls)
	}
}

func TestContextRowStreamStopsAndClosesInnerOnDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := &recordingRowStream{rows: []model.Row{{SeriesID: 1, Timestamp: 1}}}
	stream := WithContextRowStream(ctx, inner)

	cancel()
	if stream.Next() {
		t.Fatal("Next() = true, want false after context cancellation")
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("Err() = %v, want context.Canceled", stream.Err())
	}
	if inner.nextCalls != 0 {
		t.Fatalf("inner Next calls = %d, want 0", inner.nextCalls)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("inner Close calls after cancel = %d, want 1", inner.closeCalls)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if inner.closeCalls != 1 {
		t.Fatalf("inner Close calls = %d, want 1", inner.closeCalls)
	}
}

func TestAggregateStreamPropagatesSourceErrorAndClose(t *testing.T) {
	sourceErr := errors.New("source stopped")
	source := &recordingColumnStream{err: sourceErr}
	stream := NewAggregateColumnStream(source, []model.AggregateSpec{
		{Field: "value", Function: "count"},
	})

	if stream.Next() {
		t.Fatal("Next() = true, want false on source error")
	}
	if !errors.Is(stream.Err(), sourceErr) {
		t.Fatalf("Err() = %v, want source error", stream.Err())
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if source.closeCalls != 1 {
		t.Fatalf("source Close calls = %d, want 1", source.closeCalls)
	}
}

func TestContextColumnStreamPropagatesInnerErrorWhenContextActive(t *testing.T) {
	sourceErr := errors.New("inner failed")
	stream := WithContextColumnStream(context.Background(), &recordingColumnStream{err: sourceErr})

	if stream.Next() {
		t.Fatal("Next() = true, want false")
	}
	if !errors.Is(stream.Err(), sourceErr) {
		t.Fatalf("Err() = %v, want inner error", stream.Err())
	}
}

func TestContextRowStreamPropagatesInnerErrorWhenContextActive(t *testing.T) {
	sourceErr := errors.New("inner failed")
	stream := WithContextRowStream(context.Background(), &recordingRowStream{err: sourceErr})

	if stream.Next() {
		t.Fatal("Next() = true, want false")
	}
	if !errors.Is(stream.Err(), sourceErr) {
		t.Fatalf("Err() = %v, want inner error", stream.Err())
	}
}

type recordingColumnStream struct {
	columns    []model.ColumnSeries
	err        error
	index      int
	nextCalls  int
	closeCalls int
}

func (s *recordingColumnStream) Next() bool {
	s.nextCalls++
	if s.err != nil || s.index >= len(s.columns) {
		return false
	}
	s.index++
	return true
}

func (s *recordingColumnStream) Column() model.ColumnSeries {
	if s.index == 0 || s.index > len(s.columns) {
		return model.ColumnSeries{}
	}
	return s.columns[s.index-1]
}

func (s *recordingColumnStream) Err() error {
	return s.err
}

func (s *recordingColumnStream) Close() error {
	s.closeCalls++
	return nil
}

type recordingRowStream struct {
	rows       []model.Row
	err        error
	index      int
	nextCalls  int
	closeCalls int
}

func (s *recordingRowStream) Next() bool {
	s.nextCalls++
	if s.err != nil || s.index >= len(s.rows) {
		return false
	}
	s.index++
	return true
}

func (s *recordingRowStream) Row() model.Row {
	if s.index == 0 || s.index > len(s.rows) {
		return model.Row{}
	}
	return s.rows[s.index-1]
}

func (s *recordingRowStream) Err() error {
	return s.err
}

func (s *recordingRowStream) Close() error {
	s.closeCalls++
	return nil
}
