package mts

import (
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestPublicQueryBuilderBuildsQuery(t *testing.T) {
	query, err := NewQuery().
		Select("usage").
		From("metrics", "autogen", "cpu").
		Where(
			TagEq("host", "a"),
			FieldGT("usage", Float64Value(0.9)),
		).
		GroupByTime(time.Minute).
		OrderByTimeDesc().
		Limit(3).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if query.Database != "metrics" || query.RetentionPolicy != "autogen" || query.Measurement != "cpu" {
		t.Fatalf("query scope = %#v, want metrics/autogen/cpu", query)
	}
	if len(query.Fields) != 1 || query.Fields[0] != "usage" {
		t.Fatalf("fields = %#v, want usage", query.Fields)
	}
	if len(query.Predicates) != 2 {
		t.Fatalf("predicates = %d, want 2", len(query.Predicates))
	}
	if query.Order.By != QueryOrderByTime || query.Order.Direction != QuerySortDesc {
		t.Fatalf("order = %#v, want time desc", query.Order)
	}
}

func TestPublicQueryBuilderBuildsTimeRangeFromTime(t *testing.T) {
	start := time.Unix(10, 20)
	end := time.Unix(30, 40)
	query, err := NewQuery().
		Select("usage").
		From("metrics", "autogen", "cpu").
		TimeRangeTime(start, end).
		Aggregate(AggregateAvg, "usage").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if query.StartTime != start.UnixNano() || query.EndTime != end.UnixNano() {
		t.Fatalf("time range = %d/%d, want %d/%d",
			query.StartTime,
			query.EndTime,
			start.UnixNano(),
			end.UnixNano(),
		)
	}
	if query.Aggregates[0].Function != AggregateAvg {
		t.Fatalf("aggregate = %#v, want %q", query.Aggregates, AggregateAvg)
	}
}

func TestPublicQueryBuilderFromDownsamplePolicy(t *testing.T) {
	query, err := NewQuery().
		FromDownsamplePolicy(DownsamplePolicy{
			Name:              "cpu_1m",
			TargetDatabase:    "metrics",
			TargetRetention:   "rp_1m",
			TargetMeasurement: "cpu",
		}).
		Select("avg_usage").
		TimeRange(0, 60).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if query.Database != "metrics" ||
		query.RetentionPolicy != "rp_1m" ||
		query.Measurement != "cpu" {
		t.Fatalf("query source = %#v, want downsample target", query)
	}
	if query.Tags["mts_downsample_policy"] != "cpu_1m" {
		t.Fatalf("query tags = %#v, want downsample policy tag", query.Tags)
	}
}

func TestPublicQueryBuilderCoversAllPrimitivesAndModelConversion(t *testing.T) {
	query, err := NewQuery().
		Select("usage", "state").
		From("metrics", "autogen", "cpu").
		TimeRange(10, 20).
		Where(
			TagNe("host", "b"),
			TagExists("region"),
			TagIn("rack", "r1", "r2"),
			FieldEq("state", StringValue("ok")),
			FieldLTE("usage", Float64Value(0.95)),
		).
		Aggregate("mean", "usage").
		GroupByTags("host", "region").
		GroupByTime(time.Minute).
		OrderByTimeAsc().
		Limit(100).
		Offset(10).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if query.Aggregates[0].Function != "avg" {
		t.Fatalf("aggregate = %#v, want avg", query.Aggregates)
	}
	modelQuery := toModelQuery(query)
	if modelQuery.StartTime != 10 || modelQuery.EndTime != 20 {
		t.Fatalf("time range = %d/%d, want 10/20", modelQuery.StartTime, modelQuery.EndTime)
	}
	if len(modelQuery.Group.Tags) != 2 || modelQuery.Group.Window != time.Minute {
		t.Fatalf("group = %#v, want tags and 1m", modelQuery.Group)
	}
	if len(modelQuery.Predicates) != 6 {
		t.Fatalf("predicates = %d, want 6", len(modelQuery.Predicates))
	}
}

func TestPublicQueryBuilderBuildsExpressionTree(t *testing.T) {
	query, err := NewQuery().
		Select("usage").
		From("metrics", "autogen", "cpu").
		WhereExpr(OrExpr(
			PredicateQueryExpr(TagEq("host", "a")),
			AndExpr(
				PredicateQueryExpr(TagEq("host", "b")),
				NotExpr(PredicateQueryExpr(FieldLT("usage", Float64Value(0.5)))),
			),
		)).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if query.Expr.Kind != QueryExprOr {
		t.Fatalf("expr kind = %v, want OR", query.Expr.Kind)
	}
	if len(query.Predicates) != 3 {
		t.Fatalf("compat predicates = %d, want 3", len(query.Predicates))
	}
	modelQuery := toModelQuery(query)
	if modelQuery.Expr.Kind != model.QueryExprOr {
		t.Fatalf("model expr kind = %v, want OR", modelQuery.Expr.Kind)
	}
}

func TestPublicQueryBuilderCoversRemainingHelperBranches(t *testing.T) {
	query, err := NewQuery().
		Select("usage", " ").
		From("metrics", "autogen", "cpu").
		Where(FieldNe("state", StringValue("bad")), FieldGTE("usage", Float64Value(0.5))).
		WhereExpr(QueryExpr{}).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(query.Fields) != 1 || query.Fields[0] != "usage" {
		t.Fatalf("fields = %#v, want cleaned usage", query.Fields)
	}
	if len(query.Predicates) != 2 {
		t.Fatalf("predicates = %#v, want FieldNe and FieldGTE", query.Predicates)
	}

	builder := &QueryBuilder{query: Query{Measurement: "cpu"}}
	builder.Where(TagEq("host", "a"))
	query, err = builder.Build()
	if err != nil {
		t.Fatalf("Build(raw builder) error = %v", err)
	}
	if query.Tags["host"] != "a" ||
		query.Order.By != QueryOrderByTime ||
		query.Order.Direction != QuerySortAsc {
		t.Fatalf("query = %#v, want initialized tags and default time asc order", query)
	}

	builder = &QueryBuilder{err: ErrInvalidQuery}
	if _, err = builder.Build(); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Build(stored error) error = %v, want ErrInvalidQuery", err)
	}
}

func TestPublicQueryBuilderRejectsInvalidQuery(t *testing.T) {
	_, err := NewQuery().From("metrics", "autogen", "").Build()
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Build() error = %v, want ErrInvalidQuery", err)
	}
	_, err = NewQuery().From("metrics", "autogen", "cpu").Limit(-1).Build()
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("negative limit error = %v, want ErrInvalidQuery", err)
	}
	_, err = NewQuery().From("metrics", "autogen", "cpu").Offset(-1).Build()
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("negative offset error = %v, want ErrInvalidQuery", err)
	}
	_, err = NewQuery().From("metrics", "autogen", "cpu").TimeRange(20, 10).Build()
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid time range error = %v, want ErrInvalidQuery", err)
	}
}
