package querylang_test

import (
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/querylang"
)

func TestFromModelQueryNormalizesScopeAndOutput(t *testing.T) {
	spec, err := querylang.FromModelQuery(model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"usage"},
		StartTime:   10,
		EndTime:     20,
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "mean"}},
		Window:      time.Minute,
		Limit:       5,
	}, querylang.Defaults{Database: "db", RetentionPolicy: "rp"})
	if err != nil {
		t.Fatalf("FromModelQuery() error = %v", err)
	}
	if spec.Scope.Database != "db" || spec.Scope.RetentionPolicy != "rp" {
		t.Fatalf("scope = %#v, want defaults", spec.Scope)
	}
	if spec.Output.Kind != querylang.OutputColumns {
		t.Fatalf("output kind = %v, want columns", spec.Output.Kind)
	}
	if spec.Aggregates[0].Function != "avg" {
		t.Fatalf("function = %q, want avg", spec.Aggregates[0].Function)
	}
}

func TestFromModelQueryRejectsInvalidRange(t *testing.T) {
	_, err := querylang.FromModelQuery(model.Query{
		Measurement: "cpu",
		StartTime:   20,
		EndTime:     10,
	}, querylang.Defaults{})
	if !querylang.IsCode(err, querylang.ErrInvalidTimeRange) {
		t.Fatalf("error = %v, want invalid time range", err)
	}
}

func TestQuerySpecToModelQueryPreservesCompatibleFields(t *testing.T) {
	spec, err := querylang.FromModelQuery(model.Query{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          []string{"usage"},
		StartTime:       1,
		EndTime:         2,
		Aggregates:      []model.AggregateSpec{{Field: "usage", Function: "sum"}},
		Window:          time.Second,
		Limit:           10,
		Offset:          2,
	}, querylang.Defaults{})
	if err != nil {
		t.Fatalf("FromModelQuery() error = %v", err)
	}
	got := spec.ToModelQuery()
	if got.Database != "db" || got.RetentionPolicy != "rp" || got.Measurement != "cpu" {
		t.Fatalf("scope fields = %#v", got)
	}
	if got.Aggregates[0].Function != "sum" || got.Tags["host"] != "a" {
		t.Fatalf("query fields = %#v", got)
	}
}
