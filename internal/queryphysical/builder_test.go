package queryphysical_test

import (
	"testing"

	"github.com/openmts/mts/internal/queryoptimizer"
	"github.com/openmts/mts/internal/queryphysical"
	"github.com/openmts/mts/internal/queryplanner"
)

func TestBuildSelectsColumnPipelineForAggregatePlan(t *testing.T) {
	logical := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind:  queryplanner.NodeAggregate,
			Input: &queryplanner.Node{Kind: queryplanner.NodeScan},
		},
	}
	physical, err := queryphysical.Build(queryoptimizer.OptimizedPlan{Logical: logical})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if physical.Output != queryphysical.OutputColumns {
		t.Fatalf("output = %v, want columns", physical.Output)
	}
	if len(physical.Operators) != 2 {
		t.Fatalf("operator count = %d, want 2", len(physical.Operators))
	}
}

func TestBuildIncludesFilterGroupSortOperators(t *testing.T) {
	logical := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind: queryplanner.NodeLimit,
			Input: &queryplanner.Node{
				Kind: queryplanner.NodeSort,
				Input: &queryplanner.Node{
					Kind: queryplanner.NodeGroup,
					Input: &queryplanner.Node{
						Kind:  queryplanner.NodeFilter,
						Input: &queryplanner.Node{Kind: queryplanner.NodeScan},
					},
				},
			},
		},
	}
	physical, err := queryphysical.Build(queryoptimizer.OptimizedPlan{Logical: logical})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	kinds := make([]queryphysical.OperatorKind, 0, len(physical.Operators))
	for _, operator := range physical.Operators {
		kinds = append(kinds, operator.Kind)
	}
	want := []queryphysical.OperatorKind{
		queryphysical.OperatorScan,
		queryphysical.OperatorFilter,
		queryphysical.OperatorGroup,
		queryphysical.OperatorSort,
		queryphysical.OperatorLimit,
	}
	if len(kinds) != len(want) {
		t.Fatalf("operator kinds = %v, want %v", kinds, want)
	}
	for index, kind := range want {
		if kinds[index] != kind {
			t.Fatalf("operator kinds = %v, want %v", kinds, want)
		}
	}
}

func TestBuildCoversProjectAndUnsupportedNode(t *testing.T) {
	logical := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind:  queryplanner.NodeProject,
			Input: &queryplanner.Node{Kind: queryplanner.NodeScan},
		},
	}
	physical, err := queryphysical.Build(queryoptimizer.OptimizedPlan{Logical: logical})
	if err != nil {
		t.Fatalf("Build(project) error = %v", err)
	}
	if len(physical.Operators) != 2 || physical.Operators[1].Kind != queryphysical.OperatorProject {
		t.Fatalf("operators = %#v, want project pipeline", physical.Operators)
	}
	if len(physical.Operators[0].Inputs) != 0 || physical.Operators[1].Inputs[0] != "op0" {
		t.Fatalf("operator inputs = %#v, want scan without input and project from op0", physical.Operators)
	}

	_, err = queryphysical.Build(queryoptimizer.OptimizedPlan{
		Logical: queryplanner.LogicalPlan{Root: queryplanner.Node{Kind: "bad"}},
	})
	if err == nil {
		t.Fatal("Build(unsupported) error = nil, want error")
	}
}
