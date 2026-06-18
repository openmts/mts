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
