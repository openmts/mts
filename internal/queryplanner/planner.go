package queryplanner

import (
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/querylang"
)

func Build(analysis queryanalyzer.Analysis) (LogicalPlan, error) {
	spec := analysis.Spec
	scan := Node{
		Kind: NodeScan,
		Scan: &ScanNode{
			Measurement: spec.Measurement,
			TimeStart:   spec.TimeRange.Start,
			TimeEnd:     spec.TimeRange.End,
			FieldNames:  append([]string(nil), spec.Fields...),
			Tags:        cloneTags(spec.Tags),
		},
	}
	root := buildRoot(scan, spec)
	return LogicalPlan{Root: root}, nil
}

func buildRoot(scan Node, spec querylang.QuerySpec) Node {
	root := scan
	if filter := buildFilterNode(spec); filter != nil {
		input := root
		root = Node{Kind: NodeFilter, Input: &input, Filter: filter}
	}
	if len(spec.Aggregates) > 0 {
		input := root
		root = Node{
			Kind:  NodeAggregate,
			Input: &input,
			Aggregate: &AggregateNode{
				Aggregates: append([]querylang.Aggregate(nil), spec.Aggregates...),
				Window:     int64(spec.Window),
			},
		}
	} else {
		input := root
		root = Node{Kind: NodeProject, Input: &input}
	}
	if group := buildGroupNode(spec); group != nil {
		input := root
		root = Node{Kind: NodeGroup, Input: &input, Group: group}
	}
	if sort := buildSortNode(spec); sort != nil {
		input := root
		root = Node{Kind: NodeSort, Input: &input, Sort: sort}
	}
	if spec.Limit > 0 || spec.Offset > 0 {
		input := root
		root = Node{
			Kind:  NodeLimit,
			Input: &input,
			Limit: &LimitNode{Limit: spec.Limit, Offset: spec.Offset},
		}
	}
	return root
}

func buildFilterNode(spec querylang.QuerySpec) *FilterNode {
	if len(spec.Predicates) == 0 {
		return nil
	}
	predicates := make([]PredicateRef, 0, len(spec.Predicates))
	for _, predicate := range spec.Predicates {
		predicates = append(predicates, PredicateRef{
			Kind:      predicateRefKind(predicate),
			Predicate: predicate,
		})
	}
	return &FilterNode{Predicates: predicates}
}

func predicateRefKind(predicate querylang.Predicate) PredicateRefKind {
	switch predicate.Kind {
	case querylang.PredicateFieldEq,
		querylang.PredicateFieldNe,
		querylang.PredicateFieldGT,
		querylang.PredicateFieldGTE,
		querylang.PredicateFieldLT,
		querylang.PredicateFieldLTE:
		return PredicatePostFilter
	default:
		return PredicatePushdown
	}
}

func buildGroupNode(spec querylang.QuerySpec) *GroupNode {
	if len(spec.Group.Tags) == 0 && spec.Group.Window <= 0 {
		return nil
	}
	return &GroupNode{
		Tags:   append([]string(nil), spec.Group.Tags...),
		Window: int64(spec.Group.Window),
	}
}

func buildSortNode(spec querylang.QuerySpec) *SortNode {
	if spec.Order.By == querylang.OrderByNone {
		return nil
	}
	direction := SortAsc
	if spec.Order.Direction == querylang.SortDesc {
		direction = SortDesc
	}
	return &SortNode{By: SortByTime, Direction: direction}
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}
