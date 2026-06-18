package queryoptimizer

import (
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryplanner"
)

type Estimate struct {
	Shards  int
	Parts   int
	Samples int
}

type Context struct {
	Estimated Estimate
	Budget    model.QueryBudget
}

type OptimizedPlan struct {
	Logical   queryplanner.LogicalPlan
	Estimate  Estimate
	Pushdowns []string
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
	scan := findScan(plan.Root)
	if scan == nil {
		return []string{}
	}
	pushdowns := []string{}
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

func findScan(node queryplanner.Node) *queryplanner.ScanNode {
	if node.Kind == queryplanner.NodeScan {
		return node.Scan
	}
	if node.Input == nil {
		return nil
	}
	return findScan(*node.Input)
}
