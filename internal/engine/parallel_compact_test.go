package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestCompactRunsAcrossShardsWithWorkerLimit(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:                    t.TempDir(),
		ShardDuration:           time.Hour,
		MemTableMaxSamples:      1,
		MaxConcurrentCompaction: 2,
		Compaction: model.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 2,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{Measurement: "cpu", Tags: map[string]string{"h": "a"}, Timestamp: int64(1 * time.Minute), Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "cpu", Tags: map[string]string{"h": "a"}, Timestamp: int64(2 * time.Minute), Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
		{Measurement: "cpu", Tags: map[string]string{"h": "a"}, Timestamp: int64(61 * time.Minute), Fields: map[string]model.FieldValue{"v": model.Float64Value(3)}},
		{Measurement: "cpu", Tags: map[string]string{"h": "a"}, Timestamp: int64(62 * time.Minute), Fields: map[string]model.FieldValue{"v": model.Float64Value(4)}},
	}
	for _, point := range points {
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{}); err != nil {
			_ = eng.Close(ctx)
			t.Fatalf("Write() error = %v", err)
		}
	}
	if got := len(eng.snapshotShards()); got < 2 {
		_ = eng.Close(ctx)
		t.Fatalf("shard count = %d, want >=2", got)
	}
	result, err := eng.CompactWithResult(ctx)
	if err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("CompactWithResult() error = %v", err)
	}
	if result.State == compactionTaskFailed {
		_ = eng.Close(ctx)
		t.Fatalf("result failed: %#v", result)
	}
	for _, end := range []int64{int64(time.Hour), int64(2 * time.Hour)} {
		cols, _, _, err := eng.QueryWithExplain(ctx, model.Query{
			Measurement: "cpu",
			Fields:      []string{"v"},
			StartTime:   0,
			EndTime:     end,
		})
		if err != nil {
			_ = eng.Close(ctx)
			t.Fatalf("QueryWithExplain() error = %v", err)
		}
		if len(cols) == 0 {
			_ = eng.Close(ctx)
			t.Fatalf("query end=%d returned no columns", end)
		}
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDefaultParallelCompactionLimitBounded(t *testing.T) {
	limit := defaultParallelCompactionLimit()
	if limit < 1 || limit > 4 {
		t.Fatalf("defaultParallelCompactionLimit() = %d, want in [1,4]", limit)
	}
}

func TestCompactionMaxConcurrentOptionAlias(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Compaction: model.CompactionOptions{
			Enabled:       true,
			MaxConcurrent: 3,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if eng.opts.MaxConcurrentCompaction != 3 {
		_ = eng.Close(ctx)
		t.Fatalf("MaxConcurrentCompaction = %d, want 3 from Compaction.MaxConcurrent", eng.opts.MaxConcurrentCompaction)
	}
	if eng.opts.Compaction.MaxConcurrent != 3 {
		_ = eng.Close(ctx)
		t.Fatalf("Compaction.MaxConcurrent = %d, want 3", eng.opts.Compaction.MaxConcurrent)
	}
	stats := eng.MaintenanceStatsSnapshot()
	if stats.CompactionMaxConcurrent != 3 {
		_ = eng.Close(ctx)
		t.Fatalf("CompactionMaxConcurrent = %d, want 3", stats.CompactionMaxConcurrent)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
