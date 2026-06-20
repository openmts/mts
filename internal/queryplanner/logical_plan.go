package queryplanner

import "github.com/openmts/mts/internal/querylang"

type NodeKind string

const (
	NodeScan      NodeKind = "Scan"
	NodeFilter    NodeKind = "Filter"
	NodeProject   NodeKind = "Project"
	NodeAggregate NodeKind = "Aggregate"
	NodeGroup     NodeKind = "Group"
	NodeSort      NodeKind = "Sort"
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

type PredicateRefKind string

const (
	PredicatePushdown   PredicateRefKind = "pushdown"
	PredicatePostFilter PredicateRefKind = "post_filter"
)

type PredicateRef struct {
	Kind      PredicateRefKind
	Predicate querylang.Predicate
}

type FilterNode struct {
	Predicates []PredicateRef
}

type GroupNode struct {
	Tags   []string
	Window int64
}

type SortBy string

const (
	SortByTime SortBy = "time"
)

type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

type SortNode struct {
	By        SortBy
	Direction SortDirection
}

type LimitNode struct {
	Limit  int
	Offset int
}

type Node struct {
	Kind      NodeKind
	Input     *Node
	Scan      *ScanNode
	Filter    *FilterNode
	Aggregate *AggregateNode
	Group     *GroupNode
	Sort      *SortNode
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
