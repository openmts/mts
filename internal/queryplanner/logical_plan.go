package queryplanner

import "github.com/openmts/mts/internal/querylang"

type NodeKind string

const (
	NodeScan      NodeKind = "Scan"
	NodeProject   NodeKind = "Project"
	NodeAggregate NodeKind = "Aggregate"
	NodeLimit     NodeKind = "Limit"
	NodeEmpty     NodeKind = "Empty"
)

type ScanNode struct {
	Measurement string
	TimeStart   int64
	TimeEnd     int64
	FieldNames  []string
	Tags        map[string]string
}

type AggregateNode struct {
	Aggregates []querylang.Aggregate
	Window     int64
}

type LimitNode struct {
	Limit  int
	Offset int
}

type Node struct {
	Kind      NodeKind
	Input     *Node
	Scan      *ScanNode
	Aggregate *AggregateNode
	Limit     *LimitNode
}

type LogicalPlan struct {
	Root Node
}

type Explain struct {
	Root string
}

func (p LogicalPlan) Explain() Explain {
	return Explain{Root: string(p.Root.Kind)}
}
