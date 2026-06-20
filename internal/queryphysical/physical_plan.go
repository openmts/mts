package queryphysical

type OutputKind uint8

const (
	OutputColumns OutputKind = iota + 1
	OutputRows
	OutputAggregates
)

type OperatorKind string

const (
	OperatorScan      OperatorKind = "scan"
	OperatorFilter    OperatorKind = "filter"
	OperatorProject   OperatorKind = "project"
	OperatorAggregate OperatorKind = "aggregate"
	OperatorGroup     OperatorKind = "group"
	OperatorSort      OperatorKind = "sort"
	OperatorLimit     OperatorKind = "limit"
)

type Operator struct {
	ID     string
	Kind   OperatorKind
	Inputs []string
}

type PhysicalPlan struct {
	Output    OutputKind
	Operators []Operator
}
