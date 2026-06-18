package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestBuildQueryPlanExplainsCatalogAndShardPruning(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(10 * time.Minute),
			Fields: map[string]model.FieldValue{
				"load": model.Float64Value(1),
				"idle": model.Float64Value(2),
			},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   int64(2*time.Hour + 10*time.Minute),
			Fields:      map[string]model.FieldValue{"load": model.Float64Value(3)},
		},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"load"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
		Budget:      model.QueryBudget{MaxParts: 3},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if plan.Explain.SeriesCount != 1 || plan.Explain.FieldCount != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("explain counts = %#v, want one series and one field close = %v", plan.Explain, closeErr)
	}
	if plan.Explain.CandidateShards != 2 || plan.Explain.MatchedShards != 1 || plan.Explain.SkippedShards != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("shard explain = %#v, want 2 candidates, 1 matched, 1 skipped close = %v", plan.Explain, closeErr)
	}
	if !containsString(plan.Explain.Pushdowns, "series_id") ||
		!containsString(plan.Explain.Pushdowns, "field_id") ||
		!containsString(plan.Explain.Pushdowns, "shard_time") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, want series_id, field_id, shard_time close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if plan.Explain.Budget.MaxParts != 3 {
		closeErr := eng.Close(ctx)
		t.Fatalf("budget = %#v, want copied query budget close = %v", plan.Explain.Budget, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBuildQueryPlanReturnsEmptyWhenCatalogMisses(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "missing",
		Fields:      []string{"value"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if !plan.Empty || plan.Explain.SeriesCount != 0 || plan.Explain.MatchedShards != 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("plan = %#v, want empty catalog miss close = %v", plan, closeErr)
	}
	if !containsString(plan.Explain.Pushdowns, "catalog_empty") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, want catalog_empty close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
