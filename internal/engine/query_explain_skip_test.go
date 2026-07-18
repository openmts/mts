package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestQueryWithExplainExposesPageSkipStats(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
		Compression: model.CompressionOptions{
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// 写入覆盖较大时间跨度，flush 后查询窄窗，触发 page/part skip。
	for index := 0; index < 30; index++ {
		point := model.Point{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(index) * int64(time.Minute),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(index))},
		}
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{}); err != nil {
			closeTestEngine(t, ctx, eng)
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := eng.Flush(ctx); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Flush() error = %v", err)
	}
	_, explain, stats, err := eng.QueryWithExplain(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     int64(2 * time.Minute),
		Tags:        map[string]string{"host": "a"},
	})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryWithExplain() error = %v", err)
	}
	if explain.IndexRowsRead != stats.IndexRowsRead ||
		explain.IndexRowsSkipped != stats.IndexRowsSkipped ||
		explain.ValuePagesRead != stats.ValuePagesRead ||
		explain.ValuePagesSkipped != stats.ValuePagesSkipped {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("explain skip stats mismatch: explain=%+v stats=%+v", explain, stats)
	}
	closeTestEngine(t, ctx, eng)
}

func TestAttachQuerySkipStatsAddsPushdowns(t *testing.T) {
	explain := attachQuerySkipStats(model.QueryExplain{}, model.QueryStats{
		IndexRowsSkipped:  2,
		ValuePagesSkipped: 3,
		PartsSkipped:      1,
	})
	if explain.IndexRowsSkipped != 2 || explain.ValuePagesSkipped != 3 || explain.SkippedParts != 1 {
		t.Fatalf("attach stats = %+v", explain)
	}
	foundIndex, foundPage := false, false
	for _, item := range explain.Pushdowns {
		if item == "index_row_time" {
			foundIndex = true
		}
		if item == "value_page_time" {
			foundPage = true
		}
	}
	if !foundIndex || !foundPage {
		t.Fatalf("pushdowns = %v, want index/page markers", explain.Pushdowns)
	}
}
