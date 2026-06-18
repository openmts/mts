package queryplanner_test

import (
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/querylang"
	"github.com/openmts/mts/internal/queryplanner"
)

func TestPlanAggregateQueryBuildsStableLogicalTree(t *testing.T) {
	analysis := queryanalyzer.Analysis{
		Spec: querylang.QuerySpec{
			Measurement: "cpu",
			Fields:      []string{"usage"},
			TimeRange:   querylang.TimeRange{Start: 0, End: 10},
			Aggregates:  []querylang.Aggregate{{Field: "usage", Function: "avg"}},
		},
		Fields: map[string]model.FieldSchema{
			"usage": {Measurement: "cpu", Name: "usage", Type: model.FieldFloat64},
		},
	}
	plan, err := queryplanner.Build(analysis)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := plan.Root.Kind; got != queryplanner.NodeAggregate {
		t.Fatalf("root kind = %v, want aggregate", got)
	}
	if got := plan.Root.Input.Kind; got != queryplanner.NodeScan {
		t.Fatalf("input kind = %v, want scan", got)
	}
	if plan.Explain().Root != "Aggregate" {
		t.Fatalf("explain root = %q, want Aggregate", plan.Explain().Root)
	}
}
