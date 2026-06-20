package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

func TestQueryRowsAppliesStructuredTagFieldOrderAndProjection(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.2, 40),
		queryBuilderPoint("a", 2, 0.8, 55),
		queryBuilderPoint("b", 3, 0.9, 60),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateTagEq, Name: "host", StringValues: []string{"a"}},
			{Kind: model.QueryPredicateFieldGT, Name: "temp", Value: model.Int64Value(50)},
		},
		Order: model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want one row close = %v", rows, closeErr)
	}
	if rows[0].Timestamp != 2 {
		t.Fatalf("timestamp = %d, want 2", rows[0].Timestamp)
	}
	if _, ok := rows[0].Fields["usage"]; !ok {
		t.Fatalf("fields = %#v, want usage", rows[0].Fields)
	}
	if _, ok := rows[0].Fields["temp"]; ok {
		t.Fatalf("fields = %#v, temp should be projected out", rows[0].Fields)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryColumnsAppliesFieldFilterAndTimeDescOrder(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.2, 40),
		queryBuilderPoint("a", 2, 0.8, 55),
		queryBuilderPoint("a", 3, 0.9, 60),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateTagEq, Name: "host", StringValues: []string{"a"}},
			{Kind: model.QueryPredicateFieldGT, Name: "usage", Value: model.Float64Value(0.5)},
		},
		Order: model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || len(columns[0].Values) != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want one column with two values close = %v", columns, closeErr)
	}
	if columns[0].Timestamps[0] != 3 || columns[0].Timestamps[1] != 2 {
		t.Fatalf("timestamps = %v, want [3 2]", columns[0].Timestamps)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryColumnsAppliesExpressionFilterBeforeProjection(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.9, 40),
		queryBuilderPoint("a", 2, 0.4, 70),
		queryBuilderPoint("a", 3, 0.8, 80),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	expr := model.QueryExpr{
		Kind: model.QueryExprAnd,
		Children: []model.QueryExpr{
			fieldExpr("usage", model.QueryPredicateFieldGT, model.Float64Value(0.5)),
			fieldExpr("temp", model.QueryPredicateFieldGT, model.Int64Value(50)),
		},
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateFieldGT, Name: "usage", Value: model.Float64Value(0.5)},
			{Kind: model.QueryPredicateFieldGT, Name: "temp", Value: model.Int64Value(50)},
		},
		Expr: expr,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || columns[0].FieldName != "usage" {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want projected usage column close = %v", columns, closeErr)
	}
	if len(columns[0].Timestamps) != 1 || columns[0].Timestamps[0] != 3 {
		t.Fatalf("timestamps = %v, want [3]", columns[0].Timestamps)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryRowsAppliesDescendingLimitOffset(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.1, 10),
		queryBuilderPoint("a", 2, 0.2, 20),
		queryBuilderPoint("a", 3, 0.3, 30),
		queryBuilderPoint("a", 4, 0.4, 40),
		queryBuilderPoint("a", 5, 0.5, 50),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Order:       model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
		Limit:       2,
		Offset:      1,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	want := []int64{4, 3}
	if len(rows) != len(want) {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want timestamps %v close = %v", rows, want, closeErr)
	}
	for index := range want {
		if rows[index].Timestamp != want[index] {
			t.Fatalf("row %d timestamp = %d, want %d", index, rows[index].Timestamp, want[index])
		}
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryRowsAppliesOutputBudget(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.1, 10),
		queryBuilderPoint("a", 2, 0.2, 20),
		queryBuilderPoint("a", 3, 0.3, 30),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	_, err = eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Budget:      model.QueryBudget{MaxSamples: 2},
	})
	if !errors.Is(err, queryexec.ErrReadBudgetExceeded) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v, want budget exceeded close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryAggregateScansAggregateFieldWithDifferentFilterField(t *testing.T) {
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
		queryBuilderPoint("a", 1, 1, 40),
		queryBuilderPoint("a", 2, 2, 55),
		queryBuilderPoint("a", 3, 3, 60),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	expr := fieldExpr("temp", model.QueryPredicateFieldGTE, model.Int64Value(50))
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateFieldGTE, Name: "temp", Value: model.Int64Value(50)},
		},
		Expr:       expr,
		Aggregates: []model.AggregateSpec{{Field: "usage", Function: "sum"}},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want one aggregate close = %v", columns, closeErr)
	}
	if got := columns[0].Values[0].Float64; got != 5 {
		t.Fatalf("sum = %v, want 5", got)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryColumnsAppliesGroupAggregateAcrossSeries(t *testing.T) {
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
		groupAggregatePoint("east", "a", 1, 1),
		groupAggregatePoint("east", "a", 2, 2),
		groupAggregatePoint("east", "b", 1, 3),
		groupAggregatePoint("east", "b", 3, 4),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "sum"}},
		Group:       model.QueryGroup{Tags: []string{"region"}},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want one grouped result close = %v", columns, closeErr)
	}
	if columns[0].Tags["region"] != "east" || len(columns[0].Tags) != 1 {
		t.Fatalf("tags = %#v, want only region=east", columns[0].Tags)
	}
	if got := columns[0].Values[0].Float64; got != 10 {
		t.Fatalf("sum = %v, want 10", got)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryRowsAppliesExpressionOrWithoutUnsafeTagPushdown(t *testing.T) {
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
		queryBuilderPoint("a", 1, 0.1, 10),
		queryBuilderPoint("b", 2, 0.2, 20),
		queryBuilderPoint("c", 3, 0.3, 30),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	expr := model.QueryExpr{
		Kind: model.QueryExprOr,
		Children: []model.QueryExpr{
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:         model.QueryPredicateTagEq,
					Name:         "host",
					StringValues: []string{"a"},
				},
			},
			{
				Kind: model.QueryExprPredicate,
				Predicate: model.QueryPredicate{
					Kind:         model.QueryPredicateTagEq,
					Name:         "host",
					StringValues: []string{"b"},
				},
			},
		},
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateTagEq, Name: "host", StringValues: []string{"a"}},
			{Kind: model.QueryPredicateTagEq, Name: "host", StringValues: []string{"b"}},
		},
		Expr: expr,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want hosts a,b close = %v", rows, closeErr)
	}
	if rows[0].Tags["host"] != "a" || rows[1].Tags["host"] != "b" {
		t.Fatalf("hosts = %q,%q; want a,b", rows[0].Tags["host"], rows[1].Tags["host"])
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func queryBuilderPoint(host string, ts int64, usage float64, temp int64) model.Point {
	return model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": host},
		Timestamp:   ts,
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(usage),
			"temp":  model.Int64Value(temp),
		},
	}
}

func groupAggregatePoint(region string, host string, ts int64, usage float64) model.Point {
	return model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"region": region, "host": host},
		Timestamp:   ts,
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(usage),
		},
	}
}

func fieldExpr(name string, kind model.QueryPredicateKind, value model.FieldValue) model.QueryExpr {
	return model.QueryExpr{
		Kind: model.QueryExprPredicate,
		Predicate: model.QueryPredicate{
			Kind:  kind,
			Name:  name,
			Value: value,
		},
	}
}
