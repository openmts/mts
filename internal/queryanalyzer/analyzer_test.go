package queryanalyzer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/querylang"
)

func TestAnalyzeRejectsUnsupportedFunctionForFieldType(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "state", Type: model.FieldString}},
	})
	spec := querylang.QuerySpec{
		Scope:       querylang.Scope{Database: "db"},
		Measurement: "cpu",
		Fields:      []string{"state"},
		TimeRange:   querylang.TimeRange{Start: 0, End: 10},
		Aggregates:  []querylang.Aggregate{{Field: "state", Function: "sum"}},
	}
	_, err := analyzer.Analyze(context.Background(), spec)
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrFunctionTypeMismatch) {
		t.Fatalf("Analyze() error = %v, want function type mismatch", err)
	}
}

func TestAnalyzeAllowsCommercialTimeSeriesFunctionsForNumericFields(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec, err := querylang.NewBuilder().
		From("db", "rp", "cpu").
		Aggregate("rate", "usage").
		Aggregate("irate", "usage").
		Aggregate("difference", "usage").
		Aggregate("derivative", "usage").
		Aggregate("spread", "usage").
		Aggregate("median", "usage").
		Aggregate("stddev", "usage").
		Aggregate("stdvar", "usage").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, err := analyzer.Analyze(context.Background(), spec); err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func TestAnalyzeRejectsNumericTimeSeriesFunctionForStringField(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "state", Type: model.FieldString}},
	})
	spec, err := querylang.NewBuilder().
		From("db", "rp", "cpu").
		Aggregate("rate", "state").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	_, err = analyzer.Analyze(context.Background(), spec)
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrFunctionTypeMismatch) {
		t.Fatalf("Analyze() error = %v, want type mismatch", err)
	}
}

func TestAnalyzeMarksBoundaryRequirement(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec := querylang.QuerySpec{
		Scope:       querylang.Scope{Database: "db"},
		Measurement: "cpu",
		Fields:      []string{"usage"},
		TimeRange:   querylang.TimeRange{Start: 0, End: 10},
		Aggregates:  []querylang.Aggregate{{Field: "usage", Function: "first"}},
	}
	got, err := analyzer.Analyze(context.Background(), spec)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !got.RequiresBoundary {
		t.Fatal("RequiresBoundary = false, want true")
	}
}

func TestAnalyzeClassifiesWherePredicates(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("db", "rp", "cpu").
		Where(
			querylang.TagEq("host", "a"),
			querylang.TagIn("region", "east", "west"),
			querylang.FieldGT("usage", model.Float64Value(0.7)),
		).
		OrderByTimeDesc().
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got, err := analyzer.Analyze(context.Background(), spec)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(got.PushdownPredicates) != 2 {
		t.Fatalf("PushdownPredicates = %d, want 2", len(got.PushdownPredicates))
	}
	if len(got.PostFilterPredicates) != 1 {
		t.Fatalf("PostFilterPredicates = %d, want 1", len(got.PostFilterPredicates))
	}
	if got.Order.Direction != querylang.SortDesc {
		t.Fatalf("Order = %#v, want desc", got.Order)
	}
}

func TestAnalyzeClassifiesExpressionPredicates(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("db", "rp", "cpu").
		WhereExpr(querylang.Or(
			querylang.PredicateExpr(querylang.TagEq("host", "a")),
			querylang.PredicateExpr(querylang.FieldGT("usage", model.Float64Value(0.7))),
		)).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got, err := analyzer.Analyze(context.Background(), spec)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(got.PushdownPredicates) != 1 {
		t.Fatalf("PushdownPredicates = %d, want 1", len(got.PushdownPredicates))
	}
	if len(got.PostFilterPredicates) != 1 {
		t.Fatalf("PostFilterPredicates = %d, want 1", len(got.PostFilterPredicates))
	}
	if !got.RequiresPostFilterExpr {
		t.Fatal("RequiresPostFilterExpr = false, want true")
	}
}

func TestAnalyzeRejectsInvalidFieldPredicateType(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("db", "rp", "cpu").
		Where(querylang.FieldGT("usage", model.StringValue("bad"))).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	_, err = analyzer.Analyze(context.Background(), spec)
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrFunctionTypeMismatch) {
		t.Fatalf("Analyze() error = %v, want function type mismatch", err)
	}
}

func TestAnalyzeRejectsWindowGroupWithoutAggregate(t *testing.T) {
	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	})
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("db", "rp", "cpu").
		GroupByTime(time.Minute).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	_, err = analyzer.Analyze(context.Background(), spec)
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrInvalidGroup) {
		t.Fatalf("Analyze() error = %v, want invalid group", err)
	}
}

func TestAnalyzeCoversValidationAndSchemaErrors(t *testing.T) {
	validSpec := querylang.QuerySpec{Measurement: "cpu"}
	if got, err := queryanalyzer.New(nil).Analyze(context.Background(), validSpec); err != nil || len(got.Fields) != 0 {
		t.Fatalf("Analyze(nil schema) = %#v %v, want empty fields and nil error", got, err)
	}

	schemaErr := errors.New("schema down")
	_, err := queryanalyzer.New(staticSchema{err: schemaErr}).Analyze(context.Background(), validSpec)
	if err == nil || !errors.Is(err, schemaErr) {
		t.Fatalf("Analyze(schema error) error = %v, want wrapped schema error", err)
	}
	_, err = queryanalyzer.New(staticSchema{}).Analyze(context.Background(), validSpec)
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrMeasurementNotFound) {
		t.Fatalf("Analyze(empty schema) error = %v, want measurement not found", err)
	}

	analyzer := queryanalyzer.New(staticSchema{
		fields: []model.FieldSchema{
			{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64},
			{Measurement: "cpu", Name: "count", Type: model.FieldInt64},
			{Measurement: "cpu", Name: "state", Type: model.FieldString},
		},
	})
	tests := []struct {
		name string
		spec querylang.QuerySpec
		code queryanalyzer.Code
	}{
		{name: "negative window", spec: querylang.QuerySpec{Window: -1}, code: queryanalyzer.ErrInvalidWindow},
		{name: "negative group window", spec: querylang.QuerySpec{Group: querylang.GroupSpec{Window: -1}}, code: queryanalyzer.ErrInvalidWindow},
		{name: "negative limit", spec: querylang.QuerySpec{Limit: -1}, code: queryanalyzer.ErrInvalidPagination},
		{name: "field missing", spec: querylang.QuerySpec{Fields: []string{"missing"}}, code: queryanalyzer.ErrFieldNotFound},
		{
			name: "aggregate field missing",
			spec: querylang.QuerySpec{Aggregates: []querylang.Aggregate{{Field: "missing", Function: "sum"}}},
			code: queryanalyzer.ErrFieldNotFound,
		},
		{
			name: "unsupported expr",
			spec: querylang.QuerySpec{Expr: querylang.Expr{Kind: querylang.ExprKind(99)}},
			code: queryanalyzer.ErrUnsupportedFunction,
		},
		{
			name: "unsupported predicate",
			spec: querylang.QuerySpec{Predicates: []querylang.Predicate{{Kind: querylang.PredicateKind(99)}}},
			code: queryanalyzer.ErrUnsupportedFunction,
		},
		{
			name: "predicate missing field",
			spec: querylang.QuerySpec{Predicates: []querylang.Predicate{querylang.FieldEq("missing", model.Float64Value(1))}},
			code: queryanalyzer.ErrFieldNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := analyzer.Analyze(context.Background(), tt.spec)
			if !queryanalyzer.IsCode(err, tt.code) {
				t.Fatalf("Analyze() error = %v, want %s", err, tt.code)
			}
		})
	}

	okSpec := querylang.QuerySpec{
		Fields:     []string{"usage"},
		Aggregates: []querylang.Aggregate{{Field: "usage", Function: "sum"}},
		Group:      querylang.GroupSpec{Window: time.Minute},
		Predicates: []querylang.Predicate{
			querylang.FieldEq("usage", model.Int64Value(1)),
			querylang.TagExists("host"),
			querylang.TagNe("region", "west"),
		},
	}
	if _, err := analyzer.Analyze(context.Background(), okSpec); err != nil {
		t.Fatalf("Analyze(valid branches) error = %v", err)
	}
	if queryanalyzer.IsCode(nil, queryanalyzer.ErrInvalidGroup) {
		t.Fatal("IsCode(nil) = true, want false")
	}
}

type staticSchema struct {
	fields []model.FieldSchema
	err    error
}

func (s staticSchema) ListFields(context.Context, string, string) ([]model.FieldSchema, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]model.FieldSchema(nil), s.fields...), nil
}
