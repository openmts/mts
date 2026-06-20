package querylang

import (
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestQueryBuilderBuildsCommercialQueryPrimitives(t *testing.T) {
	spec, err := NewBuilder().
		Select("usage", "idle").
		From("metrics", "autogen", "cpu").
		Where(
			TagEq("host", "a"),
			TagIn("region", "east", "west"),
			FieldGTE("usage", model.Float64Value(0.75)),
		).
		TimeRange(100, 200).
		GroupByTags("host").
		GroupByTime(time.Minute).
		OrderByTimeDesc().
		Limit(10).
		Offset(5).
		Cursor("cursor-token").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if spec.Scope.Database != "metrics" || spec.Scope.RetentionPolicy != "autogen" {
		t.Fatalf("scope = %#v, want metrics/autogen", spec.Scope)
	}
	if spec.Measurement != "cpu" {
		t.Fatalf("measurement = %q, want cpu", spec.Measurement)
	}
	if len(spec.Fields) != 2 || spec.Fields[0] != "usage" || spec.Fields[1] != "idle" {
		t.Fatalf("fields = %#v, want usage,idle", spec.Fields)
	}
	if len(spec.Predicates) != 4 {
		t.Fatalf("predicates = %d, want 4", len(spec.Predicates))
	}
	if spec.Predicates[0].Kind != PredicateTagEq || spec.Predicates[2].Kind != PredicateFieldGTE {
		t.Fatalf("predicates = %#v, want tag eq and field gte", spec.Predicates)
	}
	if spec.Group.Window != time.Minute || len(spec.Group.Tags) != 1 || spec.Group.Tags[0] != "host" {
		t.Fatalf("group = %#v, want host and 1m", spec.Group)
	}
	if spec.Order.By != OrderByTime || spec.Order.Direction != SortDesc {
		t.Fatalf("order = %#v, want time desc", spec.Order)
	}
	if spec.Limit != 10 || spec.Offset != 5 {
		t.Fatalf("pagination = %d/%d, want 10/5", spec.Limit, spec.Offset)
	}
	if spec.Cursor != "cursor-token" || spec.ToModelQuery().Cursor != "cursor-token" {
		t.Fatalf("cursor = %q, want cursor-token", spec.Cursor)
	}
}

func TestQueryBuilderBuildsAggregatesAndNormalizesMean(t *testing.T) {
	spec, err := NewBuilder().
		From("metrics", "autogen", "cpu").
		Aggregate("mean", "usage").
		GroupByTime(time.Minute).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(spec.Aggregates) != 1 {
		t.Fatalf("aggregates = %d, want 1", len(spec.Aggregates))
	}
	if spec.Aggregates[0].Function != "avg" {
		t.Fatalf("aggregate function = %q, want avg", spec.Aggregates[0].Function)
	}
}

func TestQueryBuilderCoversAdditionalPredicates(t *testing.T) {
	spec, err := NewBuilder().
		Select("usage").
		From("metrics", "autogen", "cpu").
		Where(
			TagNe("host", "b"),
			TagExists("region"),
			FieldEq("state", model.StringValue("ok")),
			FieldLTE("usage", model.Float64Value(1)),
		).
		OrderByTimeAsc().
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(spec.Predicates) != 4 {
		t.Fatalf("predicates = %d, want 4", len(spec.Predicates))
	}
	modelQuery := spec.ToModelQuery()
	if len(modelQuery.Predicates) != 4 {
		t.Fatalf("model predicates = %d, want 4", len(modelQuery.Predicates))
	}
	if modelQuery.Order.Direction != model.QuerySortAsc {
		t.Fatalf("order = %#v, want asc", modelQuery.Order)
	}
}

func TestQueryBuilderBuildsExpressionTree(t *testing.T) {
	spec, err := NewBuilder().
		Select("usage").
		From("metrics", "autogen", "cpu").
		WhereExpr(Or(
			PredicateExpr(TagEq("host", "a")),
			And(
				PredicateExpr(TagEq("host", "b")),
				Not(PredicateExpr(FieldLT("usage", model.Float64Value(0.5)))),
			),
		)).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if spec.Expr.Kind != ExprOr {
		t.Fatalf("expr kind = %v, want OR", spec.Expr.Kind)
	}
	if len(spec.Expr.Children) != 2 {
		t.Fatalf("expr children = %d, want 2", len(spec.Expr.Children))
	}
	if len(spec.Predicates) != 3 {
		t.Fatalf("compat predicates = %d, want 3", len(spec.Predicates))
	}
	modelQuery := spec.ToModelQuery()
	if modelQuery.Expr.Kind != model.QueryExprOr {
		t.Fatalf("model expr kind = %v, want OR", modelQuery.Expr.Kind)
	}
}

func TestQueryBuilderRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		spec *Builder
		err  error
	}{
		{
			name: "missing measurement",
			spec: NewBuilder().Select("usage").From("metrics", "autogen", ""),
			err:  ErrInvalidMeasurement,
		},
		{
			name: "negative limit",
			spec: NewBuilder().From("metrics", "autogen", "cpu").Limit(-1),
			err:  ErrInvalidPagination,
		},
		{
			name: "negative offset",
			spec: NewBuilder().From("metrics", "autogen", "cpu").Offset(-1),
			err:  ErrInvalidPagination,
		},
		{
			name: "invalid time range",
			spec: NewBuilder().From("metrics", "autogen", "cpu").TimeRange(20, 10),
			err:  ErrInvalidTimeRange,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.spec.Build()
			if !errors.Is(err, tt.err) {
				t.Fatalf("Build() error = %v, want %v", err, tt.err)
			}
		})
	}
}
