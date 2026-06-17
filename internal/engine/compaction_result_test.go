package engine

import (
	"context"
	"testing"
	"time"

	"codeberg.org/mts/mts/internal/model"
)

func TestEngineCompactWithResultReportsTaskStatus(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []model.Point{{
			Measurement: "manual",
			Timestamp:   int64(index),
			Fields:      map[string]model.FieldValue{"value": model.Int64Value(int64(index))},
		}}, model.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write(%d) error = %v close = %v", index, err, closeErr)
		}
	}
	result, err := eng.CompactWithResult(ctx)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CompactWithResult() error = %v close = %v", err, closeErr)
	}
	if result.State != compactionTaskSucceeded || result.InputParts < 2 || result.OutputParts == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("result = %#v, want successful affected compact close = %v", result, closeErr)
	}
	if result.Duration <= 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("Duration = %s, want positive close = %v", result.Duration, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestShardCompactWithResultReportsNoop(t *testing.T) {
	shard, _, err := OpenShard(ShardOptions{
		Dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	result, err := shard.CompactWithResult()
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("CompactWithResult() error = %v close = %v", err, closeErr)
	}
	if result.State != compactionTaskNoop || result.InputParts != 0 || result.OutputParts != 0 {
		closeErr := shard.Close()
		t.Fatalf("result = %#v, want noop close = %v", result, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
