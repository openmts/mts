package querylang

import (
	"strings"

	"github.com/openmts/mts/internal/collections"
	"github.com/openmts/mts/internal/model"
)

func FromModelQuery(query model.Query, defaults Defaults) (QuerySpec, error) {
	spec := QuerySpec{
		Scope: Scope{
			Database:        firstNonEmpty(query.Database, defaults.Database),
			RetentionPolicy: firstNonEmpty(query.RetentionPolicy, defaults.RetentionPolicy),
		},
		Measurement: query.Measurement,
		Tags:        cloneTags(query.Tags),
		Fields:      append([]string(nil), query.Fields...),
		TimeRange: TimeRange{
			Start: query.StartTime,
			End:   query.EndTime,
		},
		Predicates: convertPredicates(query.Predicates),
		Expr:       convertExpr(query.Expr),
		Aggregates: convertAggregates(query.Aggregates),
		Window:     query.Window,
		Group: GroupSpec{
			Tags:   append([]string(nil), query.Group.Tags...),
			Window: query.Group.Window,
		},
		Order: OrderSpec{
			By:        OrderBy(query.Order.By),
			Direction: SortDirection(query.Order.Direction),
		},
		Limit:  query.Limit,
		Offset: query.Offset,
		Output: Output{Kind: defaults.Output},
	}
	if spec.Output.Kind == 0 {
		spec.Output.Kind = OutputColumns
	}
	if err := validate(spec); err != nil {
		return QuerySpec{}, err
	}
	return spec, nil
}

func convertExpr(expr model.QueryExpr) Expr {
	out := Expr{
		Kind:      ExprKind(expr.Kind),
		Predicate: convertPredicate(expr.Predicate),
		Children:  make([]Expr, 0, len(expr.Children)),
	}
	for _, child := range expr.Children {
		out.Children = append(out.Children, convertExpr(child))
	}
	return out
}

func validate(spec QuerySpec) error {
	if strings.TrimSpace(spec.Measurement) == "" {
		return newError(ErrInvalidMeasurement, "measurement is required")
	}
	if spec.TimeRange.End != 0 && spec.TimeRange.Start > spec.TimeRange.End {
		return newError(ErrInvalidTimeRange, "start time must be less than or equal to end time")
	}
	if spec.Limit < 0 || spec.Offset < 0 {
		return newError(ErrInvalidPagination, "limit and offset must be greater than or equal to zero")
	}
	return nil
}

func convertPredicates(predicates []model.QueryPredicate) []Predicate {
	out := make([]Predicate, 0, len(predicates))
	for _, predicate := range predicates {
		out = append(out, convertPredicate(predicate))
	}
	return out
}

func convertPredicate(predicate model.QueryPredicate) Predicate {
	return Predicate{
		Kind:         PredicateKind(predicate.Kind),
		Name:         predicate.Name,
		StringValues: append([]string(nil), predicate.StringValues...),
		Value:        predicate.Value,
		Start:        predicate.Start,
		End:          predicate.End,
	}
}

func convertAggregates(specs []model.AggregateSpec) []Aggregate {
	out := make([]Aggregate, 0, len(specs))
	for _, spec := range specs {
		out = append(out, Aggregate{
			Field:    spec.Field,
			Function: normalizeFunction(spec.Function),
		})
	}
	return out
}

func normalizeFunction(function string) string {
	normalized := strings.ToLower(strings.TrimSpace(function))
	if normalized == "mean" {
		return "avg"
	}
	return normalized
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	return collections.CloneMap(tags)
}

func firstNonEmpty(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
