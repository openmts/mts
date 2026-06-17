package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"codeberg.org/mts/mts/internal/model"
)

func TestQueryColumnIteratorReturnsDeadlineDuringCatalog(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Nanosecond))
	defer cancel()

	_, err = eng.QueryColumnIterator(deadlineCtx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumnIterator() error = %v, want deadline exceeded close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryRowIteratorReturnsContextErrorAfterCreation(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"value": model.Float64Value(1)},
	}}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}

	queryCtx, cancel := context.WithCancel(ctx)
	iter, err := eng.QueryRowIterator(queryCtx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     10,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRowIterator() error = %v close = %v", err, closeErr)
	}
	cancel()
	if iter.Next() {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("Next() = true after cancel, want false close = %v", closeErr)
	}
	if !errors.Is(iter.Err(), context.Canceled) {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("Err() = %v, want context.Canceled close = %v", iter.Err(), closeErr)
	}
	if err := iter.Close(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("iterator Close() error = %v engine close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
