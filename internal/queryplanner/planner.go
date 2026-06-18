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
	var root Node
	if len(spec.Aggregates) > 0 {
		root = Node{
			Kind:  NodeAggregate,
			Input: &scan,
			Aggregate: &AggregateNode{
				Aggregates: append([]querylang.Aggregate(nil), spec.Aggregates...),
				Window:     int64(spec.Window),
			},
		}
	} else {
		root = Node{Kind: NodeProject, Input: &scan}
	}
	if spec.Limit > 0 || spec.Offset > 0 {
		root = Node{
			Kind:  NodeLimit,
			Input: &root,
			Limit: &LimitNode{Limit: spec.Limit, Offset: spec.Offset},
		}
	}
	return root
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
