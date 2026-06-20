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

func TestPlanQueryBuildsFilterGroupSortLimitTree(t *testing.T) {
	analysis := queryanalyzer.Analysis{
		Spec: querylang.QuerySpec{
			Measurement: "cpu",
			Fields:      []string{"usage"},
			Predicates: []querylang.Predicate{
				querylang.TagEq("host", "a"),
				querylang.FieldGT("usage", model.Float64Value(0.7)),
			},
			Group: querylang.GroupSpec{Tags: []string{"host"}},
			Order: querylang.OrderSpec{
				By:        querylang.OrderByTime,
				Direction: querylang.SortDesc,
			},
			Limit: 5,
		},
		Fields: map[string]model.FieldSchema{
			"usage": {Measurement: "cpu", Name: "usage", Type: model.FieldFloat64},
		},
	}
	plan, err := queryplanner.Build(analysis)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	assertPlanContains(t, plan.Root, queryplanner.NodeFilter)
	assertPlanContains(t, plan.Root, queryplanner.NodeGroup)
	assertPlanContains(t, plan.Root, queryplanner.NodeSort)
	assertPlanContains(t, plan.Root, queryplanner.NodeLimit)
}

func assertPlanContains(t *testing.T, node queryplanner.Node, kind queryplanner.NodeKind) {
	t.Helper()
	if node.Kind == kind {
		return
	}
	if node.Input == nil {
		t.Fatalf("plan does not contain %s", kind)
	}
	assertPlanContains(t, *node.Input, kind)
}
