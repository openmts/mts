package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
)

func TestEstimateQueryCostScalesWithTimeWindow(t *testing.T) {
	explain := model.QueryExplain{
		SeriesCount:     2,
		FieldCount:      3,
		MatchedShards:   1,
		CandidateShards: 1,
	}
	short := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(time.Hour),
	}, explain)
	long := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(10 * time.Hour),
	}, explain)
	if short.EstimatedSamples <= 0 {
		t.Fatalf("short estimate = %d, want >0", short.EstimatedSamples)
	}
	if long.EstimatedSamples <= short.EstimatedSamples {
		t.Fatalf("long estimate = %d, short = %d, want long > short", long.EstimatedSamples, short.EstimatedSamples)
	}
}

func TestEstimateQueryCostPrefersPartRows(t *testing.T) {
	heuristic := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(time.Hour),
	}, model.QueryExplain{
		SeriesCount:   2,
		FieldCount:    1,
		MatchedShards: 1,
	})
	calibrated := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(time.Hour),
	}, model.QueryExplain{
		SeriesCount:       2,
		FieldCount:        1,
		MatchedShards:     1,
		MatchedParts:      1,
		EstimatedPartRows: 1000,
	})
	if calibrated.EstimatedPartRows != 1000 {
		t.Fatalf("EstimatedPartRows = %d, want 1000", calibrated.EstimatedPartRows)
	}
	if calibrated.EstimatedSamples < 1000 {
		t.Fatalf("calibrated estimate = %d, want >= 1000", calibrated.EstimatedSamples)
	}
	if calibrated.EstimatedSamples <= heuristic.EstimatedSamples {
		t.Fatalf("calibrated = %d heuristic = %d, want calibrated > heuristic", calibrated.EstimatedSamples, heuristic.EstimatedSamples)
	}
	if calibrated.MatchedParts != 1 {
		t.Fatalf("MatchedParts = %d, want 1", calibrated.MatchedParts)
	}
}

func TestProportionalPartRowsScalesWithOverlap(t *testing.T) {
	meta := sstable.PartMeta{MinTime: 0, MaxTime: 100, RowsCount: 100}
	full := proportionalPartRows(meta, 0, 100)
	half := proportionalPartRows(meta, 0, 50)
	none := proportionalPartRows(meta, 200, 300)
	if full != 100 {
		t.Fatalf("full = %d, want 100", full)
	}
	if half != 50 {
		t.Fatalf("half = %d, want 50", half)
	}
	if none != 0 {
		t.Fatalf("none = %d, want 0", none)
	}
}

func TestBuildQueryPlanUsesPartStatsForCost(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1, // 强制 flush 形成 part
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// 写入 4 个样本，触发至少 1 个 part。
	for i := 0; i < 4; i++ {
		if err := eng.Write(ctx, []model.Point{{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(i + 1),
			Fields:      map[string]model.FieldValue{"load": model.Float64Value(float64(i))},
		}}, model.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"load"},
		StartTime:   0,
		EndTime:     10,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if plan.Explain.MatchedParts == 0 && plan.Explain.EstimatedPartRows == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("part stats empty: %#v close = %v", plan.Explain, closeErr)
	}
	if plan.Explain.Cost.EstimatedSamples <= 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("EstimatedSamples = %d, want >0 close = %v", plan.Explain.Cost.EstimatedSamples, closeErr)
	}
	if !containsString(plan.Explain.Pushdowns, "part_time") && plan.Explain.MatchedParts > 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, want part_time close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
