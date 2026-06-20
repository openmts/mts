package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
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
	if plan.Explain.ReadEpoch <= 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("read epoch = %d, want positive epoch close = %v", plan.Explain.ReadEpoch, closeErr)
	}
	if plan.Explain.Cost.SeriesCount != 1 ||
		plan.Explain.Cost.FieldCount != 1 ||
		plan.Explain.Cost.MatchedShards != 1 ||
		plan.Explain.Cost.PlanClass != "scan" {
		closeErr := eng.Close(ctx)
		t.Fatalf("cost = %#v, want scan cost metadata close = %v", plan.Explain.Cost, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryWithExplainBindsReadEpochIntoStats(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{queryPlanPoint("a", 10)}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	_, explain, stats, err := eng.QueryWithExplain(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryWithExplain() error = %v close = %v", err, closeErr)
	}
	if explain.ReadEpoch <= 0 || stats.ReadEpoch != explain.ReadEpoch {
		closeErr := eng.Close(ctx)
		t.Fatalf("epoch explain=%d stats=%d close=%v", explain.ReadEpoch, stats.ReadEpoch, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryRowsResumesFromCursor(t *testing.T) {
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
		queryPlanPoint("a", 10),
		queryPlanPoint("b", 10),
		queryPlanPoint("c", 10),
		queryPlanPoint("a", 9),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	firstPage, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Order:       model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
		Limit:       2,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows(first page) error = %v close = %v", err, closeErr)
	}
	if len(firstPage) != 2 || firstPage[0].Timestamp != 10 || firstPage[1].Timestamp != 10 {
		closeErr := eng.Close(ctx)
		t.Fatalf("firstPage = %#v, want two newest rows close = %v", firstPage, closeErr)
	}
	token, err := queryexec.EncodeCursor(queryexec.CursorFromRow(
		firstPage[len(firstPage)-1],
		model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
	))
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("EncodeCursor() error = %v close = %v", err, closeErr)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Order:       model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
		Cursor:      token,
		Limit:       2,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows(next page) error = %v close = %v", err, closeErr)
	}
	if len(rows) != 2 || rows[0].Timestamp != 10 || rows[1].Timestamp != 9 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want remaining ts 10 row then ts 9 close = %v", rows, closeErr)
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

func TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   1,
			Fields: map[string]model.FieldValue{
				"usage": model.Float64Value(0.9),
				"temp":  model.Int64Value(40),
			},
		},
	}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	expr := model.QueryExpr{
		Kind: model.QueryExprOr,
		Children: []model.QueryExpr{
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "usage",
					Value: model.Float64Value(0.8),
				},
			},
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "temp",
					Value: model.Int64Value(80),
				},
			},
		},
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateFieldGT, Name: "usage", Value: model.Float64Value(0.8)},
			{Kind: model.QueryPredicateFieldGT, Name: "temp", Value: model.Int64Value(80)},
		},
		Expr: expr,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if len(plan.FieldPredicates) != 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("FieldPredicates = %#v, want none for OR expr close = %v", plan.FieldPredicates, closeErr)
	}
	if containsString(plan.Explain.Pushdowns, "field_page_stats") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, field_page_stats should be disabled for OR expr close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if !containsString(plan.Explain.Pushdowns, "post_filter") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, want post_filter close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBuildQueryPlanPushesDownTagOnlyOrExpression(t *testing.T) {
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
		queryPlanPoint("a", 1),
		queryPlanPoint("b", 2),
		queryPlanPoint("c", 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	expr := model.QueryExpr{
		Kind: model.QueryExprOr,
		Children: []model.QueryExpr{
			tagExpr("host", "a"),
			tagExpr("host", "b"),
		},
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Expr:        expr,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if plan.Explain.SeriesCount != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("SeriesCount = %d, want 2 close = %v", plan.Explain.SeriesCount, closeErr)
	}
	if !containsString(plan.Explain.Pushdowns, "tag_expr") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, want tag_expr close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBuildQueryPlanKeepsMixedFieldOrExpressionUnpruned(t *testing.T) {
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
		queryPlanPoint("a", 1),
		queryPlanPoint("b", 2),
		queryPlanPoint("c", 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	expr := model.QueryExpr{
		Kind: model.QueryExprOr,
		Children: []model.QueryExpr{
			tagExpr("host", "a"),
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "usage",
					Value: model.Float64Value(2),
				},
			},
		},
	}
	plan, err := eng.BuildQueryPlan(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateFieldGT, Name: "usage", Value: model.Float64Value(2)},
		},
		Expr: expr,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("BuildQueryPlan() error = %v close = %v", err, closeErr)
	}
	if plan.Explain.SeriesCount != 3 {
		closeErr := eng.Close(ctx)
		t.Fatalf("SeriesCount = %d, want all 3 close = %v", plan.Explain.SeriesCount, closeErr)
	}
	if containsString(plan.Explain.Pushdowns, "tag_expr") {
		closeErr := eng.Close(ctx)
		t.Fatalf("pushdowns = %v, tag_expr should be disabled close = %v", plan.Explain.Pushdowns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func queryPlanPoint(host string, usage float64) model.Point {
	return model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": host},
		Timestamp:   int64(usage),
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(usage),
		},
	}
}

func tagExpr(name string, value string) model.QueryExpr {
	return model.QueryExpr{
		Kind: model.QueryExprPredicate,
		Predicate: model.QueryPredicate{
			Kind:         model.QueryPredicateTagEq,
			Name:         name,
			StringValues: []string{value},
		},
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
