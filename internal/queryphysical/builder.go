package queryphysical

import (
	"fmt"

	"github.com/openmts/mts/internal/queryoptimizer"
	"github.com/openmts/mts/internal/queryplanner"
)

func Build(plan queryoptimizer.OptimizedPlan) (PhysicalPlan, error) {
	var operators []Operator
	if err := appendOperators(plan.Logical.Root, &operators); err != nil {
		return PhysicalPlan{}, err
	}
	return PhysicalPlan{
		Output:    chooseOutput(plan.Logical.Root),
		Operators: operators,
	}, nil
}

func appendOperators(node queryplanner.Node, operators *[]Operator) error {
	if node.Input != nil {
		if err := appendOperators(*node.Input, operators); err != nil {
			return err
		}
	}
	operator, err := buildOperator(node, len(*operators))
	if err != nil {
		return err
	}
	*operators = append(*operators, operator)
	return nil
}

func buildOperator(node queryplanner.Node, index int) (Operator, error) {
	id := fmt.Sprintf("op%d", index)
	switch node.Kind {
	case queryplanner.NodeScan:
		return Operator{ID: id, Kind: OperatorScan}, nil
	case queryplanner.NodeProject:
		return Operator{ID: id, Kind: OperatorProject, Inputs: previousInput(index)}, nil
	case queryplanner.NodeAggregate:
		return Operator{ID: id, Kind: OperatorAggregate, Inputs: previousInput(index)}, nil
	case queryplanner.NodeLimit:
		return Operator{ID: id, Kind: OperatorLimit, Inputs: previousInput(index)}, nil
	default:
		return Operator{}, fmt.Errorf("unsupported logical node %q", node.Kind)
	}
}

func previousInput(index int) []string {
	if index == 0 {
		return []string{}
	}
	return []string{fmt.Sprintf("op%d", index-1)}
}

func chooseOutput(root queryplanner.Node) OutputKind {
	if contains(root, queryplanner.NodeAggregate) {
		return OutputColumns
	}
	return OutputRows
}

func contains(node queryplanner.Node, kind queryplanner.NodeKind) bool {
	if node.Kind == kind {
		return true
	}
	if node.Input == nil {
		return false
	}
	return contains(*node.Input, kind)
}
