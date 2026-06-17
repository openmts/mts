package engine

import (
	"strings"

	"codeberg.org/mts/mts/internal/model"
)

func queryBoundaryMode(query model.Query) model.QueryBoundaryMode {
	if query.Window > 0 || len(query.Aggregates) == 0 {
		return model.QueryBoundaryNone
	}
	mode := model.QueryBoundaryNone
	for _, spec := range query.Aggregates {
		switch strings.ToLower(spec.Function) {
		case "first":
			mode = mergeBoundaryMode(mode, model.QueryBoundaryFirst)
		case "last":
			mode = mergeBoundaryMode(mode, model.QueryBoundaryLast)
		default:
			return model.QueryBoundaryNone
		}
	}
	return mode
}

func mergeBoundaryMode(left model.QueryBoundaryMode, right model.QueryBoundaryMode) model.QueryBoundaryMode {
	if left == model.QueryBoundaryNone {
		return right
	}
	if left == right {
		return left
	}
	return model.QueryBoundaryBoth
}
