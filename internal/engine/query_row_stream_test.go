package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestQueryRowIteratorDoesNotMaterializeAllColumnsBeforeNext(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   1,
			Fields: map[string]model.FieldValue{
				"load":  model.Float64Value(1),
				"state": model.StringValue("ok"),
			},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   1,
			Fields: map[string]model.FieldValue{
				"load":  model.Float64Value(2),
				"state": model.StringValue("ok"),
			},
		},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	decorated := 0
	previousHook := decorateColumnDataHook
	decorateColumnDataHook = func() {
		decorated++
	}
	defer func() {
		decorateColumnDataHook = previousHook
	}()

	iter, err := eng.QueryRowIterator(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     10,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRowIterator() error = %v close = %v", err, closeErr)
	}
	if decorated != 0 {
		closeErr := errorsJoin(iter.Close(), eng.Close(ctx))
		t.Fatalf("decorated at iterator creation = %d, want 0 close = %v", decorated, closeErr)
	}
	if !iter.Next() {
		closeErr := errorsJoin(iter.Close(), eng.Close(ctx))
		t.Fatalf("Next() = false, want first row err=%v close = %v", iter.Err(), closeErr)
	}
	if decorated >= 4 {
		closeErr := errorsJoin(iter.Close(), eng.Close(ctx))
		t.Fatalf("decorated after first row = %d, want less than all 4 columns close = %v", decorated, closeErr)
	}
	if err := iter.Close(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("iterator Close() error = %v engine close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
