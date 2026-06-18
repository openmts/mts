package queryanalyzer_test

import (
	"context"
	"testing"

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

type staticSchema struct {
	fields []model.FieldSchema
}

func (s staticSchema) ListFields(context.Context, string, string) ([]model.FieldSchema, error) {
	return append([]model.FieldSchema(nil), s.fields...), nil
}
