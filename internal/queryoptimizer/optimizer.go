package queryoptimizer

import (
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryplanner"
)

type Estimate struct {
	Shards     int
	Parts      int
	Samples    int
	Series     int
	Fields     int
	Limit      int
	Offset     int
	Window     int64
	Ordered    bool
	HasCursor  bool
	Aggregated bool
}

type Context struct {
	Estimated Estimate
	Budget    model.QueryBudget
}

type OptimizedPlan struct {
	Logical   queryplanner.LogicalPlan
	Estimate  Estimate
	Pushdowns []string
	Strategy  string
}

func Optimize(plan queryplanner.LogicalPlan, ctx Context) (OptimizedPlan, error) {
	if exceedsBudget(ctx.Estimated.Shards, ctx.Budget.MaxShards) {
		return OptimizedPlan{}, newError(ErrBudgetExceeded, "estimated shards exceeds query budget")
	}
	if exceedsBudget(ctx.Estimated.Parts, ctx.Budget.MaxParts) {
		return OptimizedPlan{}, newError(ErrBudgetExceeded, "estimated parts exceeds query budget")
	}
	if exceedsBudget(ctx.Estimated.Samples, ctx.Budget.MaxSamples) {
		return OptimizedPlan{}, newError(ErrBudgetExceeded, "estimated samples exceeds query budget")
	}
	return OptimizedPlan{
		Logical:   plan,
		Estimate:  ctx.Estimated,
		Pushdowns: detectPushdowns(plan),
		Strategy:  chooseStrategy(plan, ctx.Estimated),
	}, nil
}

func (p OptimizedPlan) HasPushdown(name string) bool {
	for _, pushdown := range p.Pushdowns {
		if pushdown == name {
			return true
		}
	}
	return false
}

func exceedsBudget(actual int, limit int) bool {
	return limit > 0 && actual > limit
}

func detectPushdowns(plan queryplanner.LogicalPlan) []string {
	pushdowns := detectNodePushdowns(plan.Root, nil)
	scan := findScan(plan.Root)
	if scan == nil {
		return pushdowns
	}
	if len(scan.FieldNames) > 0 {
		pushdowns = append(pushdowns, "field_id")
	}
	if scan.TimeStart != 0 || scan.TimeEnd != 0 {
		pushdowns = append(pushdowns, "time_range")
	}
	if len(scan.Tags) > 0 {
		pushdowns = append(pushdowns, "series_id")
	}
	return pushdowns
}

func chooseStrategy(plan queryplanner.LogicalPlan, estimate Estimate) string {
	if !hasNode(plan.Root, queryplanner.NodeScan) {
		return "empty"
	}
	if estimate.Aggregated || hasNode(plan.Root, queryplanner.NodeAggregate) ||
		hasNode(plan.Root, queryplanner.NodeGroup) {
		return "aggregate"
	}
	if estimate.Limit > 0 || estimate.HasCursor || hasNode(plan.Root, queryplanner.NodeLimit) {
		return "bounded_scan"
	}
	if estimate.Ordered || hasNode(plan.Root, queryplanner.NodeSort) {
		return "ordered_scan"
	}
	return "scan"
}

func hasNode(node queryplanner.Node, kind queryplanner.NodeKind) bool {
	if node.Kind == kind {
		return true
	}
	if node.Input == nil {
		return false
	}
	return hasNode(*node.Input, kind)
}

func detectNodePushdowns(node queryplanner.Node, out []string) []string {
	switch node.Kind {
	case queryplanner.NodeFilter:
		for _, predicate := range node.Filter.Predicates {
			if predicate.Kind == queryplanner.PredicatePostFilter {
				out = append(out, "post_filter")
				continue
			}
			out = append(out, "predicate")
		}
	case queryplanner.NodeGroup:
		if len(node.Group.Tags) > 0 {
			out = append(out, "group_tags")
		}
		if node.Group.Window > 0 {
			out = append(out, "group_time")
		}
	case queryplanner.NodeSort:
		if node.Sort.By == queryplanner.SortByTime {
			out = append(out, "order_time")
		}
	case queryplanner.NodeLimit:
		out = append(out, "limit")
	}
	if node.Input == nil {
		return out
	}
	return detectNodePushdowns(*node.Input, out)
}

func findScan(node queryplanner.Node) *queryplanner.ScanNode {
	if node.Kind == queryplanner.NodeScan {
		return node.Scan
	}
	if node.Input == nil {
		return nil
	}
	return findScan(*node.Input)
}
