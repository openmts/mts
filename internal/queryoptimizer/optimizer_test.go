package queryoptimizer_test

import (
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryoptimizer"
	"github.com/openmts/mts/internal/queryplanner"
)

func TestOptimizeRecordsPushdownsAndRejectsExcessCost(t *testing.T) {
	plan := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind: queryplanner.NodeScan,
			Scan: &queryplanner.ScanNode{
				Measurement: "cpu",
				TimeStart:   0,
				TimeEnd:     10,
				FieldNames:  []string{"usage"},
			},
		},
	}
	_, err := queryoptimizer.Optimize(plan, queryoptimizer.Context{
		Estimated: queryoptimizer.Estimate{Shards: 2, Parts: 5, Samples: 100},
		Budget:    model.QueryBudget{MaxParts: 1},
	})
	if !queryoptimizer.IsCode(err, queryoptimizer.ErrBudgetExceeded) {
		t.Fatalf("Optimize() error = %v, want budget exceeded", err)
	}

	optimized, err := queryoptimizer.Optimize(plan, queryoptimizer.Context{
		Estimated: queryoptimizer.Estimate{Shards: 1, Parts: 1, Samples: 10},
	})
	if err != nil {
		t.Fatalf("Optimize(valid) error = %v", err)
	}
	if !optimized.HasPushdown("field_id") || !optimized.HasPushdown("time_range") {
		t.Fatalf("pushdowns = %v, want field_id and time_range", optimized.Pushdowns)
	}
}
