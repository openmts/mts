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
	if optimized.Strategy != "scan" {
		t.Fatalf("strategy = %q, want scan", optimized.Strategy)
	}
}

func TestOptimizeRecordsFilterGroupOrderPushdowns(t *testing.T) {
	plan := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind: queryplanner.NodeLimit,
			Input: &queryplanner.Node{
				Kind: queryplanner.NodeSort,
				Sort: &queryplanner.SortNode{By: queryplanner.SortByTime, Direction: queryplanner.SortDesc},
				Input: &queryplanner.Node{
					Kind:  queryplanner.NodeGroup,
					Group: &queryplanner.GroupNode{Tags: []string{"host"}},
					Input: &queryplanner.Node{
						Kind: queryplanner.NodeFilter,
						Filter: &queryplanner.FilterNode{
							Predicates: []queryplanner.PredicateRef{
								{Kind: queryplanner.PredicatePostFilter},
							},
						},
						Input: &queryplanner.Node{Kind: queryplanner.NodeScan},
					},
				},
			},
		},
	}
	optimized, err := queryoptimizer.Optimize(plan, queryoptimizer.Context{})
	if err != nil {
		t.Fatalf("Optimize() error = %v", err)
	}
	for _, pushdown := range []string{"post_filter", "group_tags", "order_time", "limit"} {
		if !optimized.HasPushdown(pushdown) {
			t.Fatalf("pushdowns = %v, want %s", optimized.Pushdowns, pushdown)
		}
	}
	if optimized.Strategy != "aggregate" {
		t.Fatalf("strategy = %q, want aggregate for grouped plan", optimized.Strategy)
	}
}

func TestOptimizeChoosesBoundedScanForCursorOrLimit(t *testing.T) {
	plan := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind: queryplanner.NodeScan,
			Scan: &queryplanner.ScanNode{Measurement: "cpu"},
		},
	}
	optimized, err := queryoptimizer.Optimize(plan, queryoptimizer.Context{
		Estimated: queryoptimizer.Estimate{HasCursor: true, Limit: 100},
	})
	if err != nil {
		t.Fatalf("Optimize() error = %v", err)
	}
	if optimized.Strategy != "bounded_scan" {
		t.Fatalf("strategy = %q, want bounded_scan", optimized.Strategy)
	}
}

func TestOptimizeCoversBudgetsStrategiesAndErrors(t *testing.T) {
	scanPlan := queryplanner.LogicalPlan{
		Root: queryplanner.Node{
			Kind: queryplanner.NodeScan,
			Scan: &queryplanner.ScanNode{
				Tags: map[string]string{"host": "a"},
			},
		},
	}
	for _, ctx := range []queryoptimizer.Context{
		{Estimated: queryoptimizer.Estimate{Shards: 2}, Budget: model.QueryBudget{MaxShards: 1}},
		{Estimated: queryoptimizer.Estimate{Samples: 2}, Budget: model.QueryBudget{MaxSamples: 1}},
	} {
		if _, err := queryoptimizer.Optimize(scanPlan, ctx); !queryoptimizer.IsCode(err, queryoptimizer.ErrBudgetExceeded) {
			t.Fatalf("Optimize(%#v) error = %v, want budget exceeded", ctx, err)
		}
	}

	ordered, err := queryoptimizer.Optimize(scanPlan, queryoptimizer.Context{
		Estimated: queryoptimizer.Estimate{Ordered: true},
	})
	if err != nil {
		t.Fatalf("Optimize(ordered) error = %v", err)
	}
	if ordered.Strategy != "ordered_scan" || !ordered.HasPushdown("series_id") || ordered.HasPushdown("missing") {
		t.Fatalf("ordered plan = %#v, want ordered_scan with series_id only", ordered)
	}

	aggregate, err := queryoptimizer.Optimize(scanPlan, queryoptimizer.Context{
		Estimated: queryoptimizer.Estimate{Aggregated: true},
	})
	if err != nil {
		t.Fatalf("Optimize(aggregated) error = %v", err)
	}
	if aggregate.Strategy != "aggregate" {
		t.Fatalf("aggregate strategy = %q, want aggregate", aggregate.Strategy)
	}

	empty, err := queryoptimizer.Optimize(queryplanner.LogicalPlan{
		Root: queryplanner.Node{Kind: queryplanner.NodeLimit},
	}, queryoptimizer.Context{})
	if err != nil {
		t.Fatalf("Optimize(empty) error = %v", err)
	}
	if empty.Strategy != "empty" {
		t.Fatalf("empty strategy = %q, want empty", empty.Strategy)
	}
	if queryoptimizer.IsCode(nil, queryoptimizer.ErrBudgetExceeded) {
		t.Fatal("IsCode(nil) = true, want false")
	}
}
