package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEngineRetentionDeletesExpiredSamplesInsideActiveShard(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		Retention:          30 * time.Minute,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// 同一小时分片内：10min 旧点应过期，50min 新点应保留。
	// now=70min，cutoff=40min → 删除 [0,40min) 内数据。
	if err := eng.Write(ctx, []model.Point{
		{
			Measurement: "cpu",
			Timestamp:   int64(10 * time.Minute),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Timestamp:   int64(50 * time.Minute),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(2)},
		},
	}, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}

	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(70*time.Minute))); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("ApplyRetention() error = %v", err)
	}

	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("row count = %d, want 1 retained sample", len(rows))
	}
	if rows[0].Fields["v"].Float64 != 2 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("retained value = %v, want 2", rows[0].Fields["v"].Float64)
	}
	closeTestEngine(t, ctx, eng)
}
