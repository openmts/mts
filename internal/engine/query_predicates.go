package engine

import (
	"context"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/model"
)

func normalizeStructuredQuery(query model.Query) model.Query {
	if query.Expr.Kind != model.QueryExprNone && !queryExprAllowsFlatPredicates(query.Expr) {
		return query
	}
	for _, predicate := range query.Predicates {
		switch predicate.Kind {
		case model.QueryPredicateTimeRange:
			query.StartTime = predicate.Start
			query.EndTime = predicate.End
		case model.QueryPredicateTagEq:
			if len(predicate.StringValues) > 0 {
				query.Tags[predicate.Name] = predicate.StringValues[0]
			}
		default:
		}
	}
	return query
}

func queryExprAllowsFlatPredicates(expr model.QueryExpr) bool {
	switch expr.Kind {
	case model.QueryExprNone, model.QueryExprPredicate:
		return true
	case model.QueryExprAnd:
		for _, child := range expr.Children {
			if !queryExprAllowsFlatPredicates(child) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func fieldPredicates(predicates []model.QueryPredicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		if isFieldPredicate(predicate.Kind) {
			out = append(out, predicate)
		}
	}
	return out
}

func andExprFromPredicates(predicates []model.QueryPredicate) model.QueryExpr {
	if len(predicates) == 0 {
		return model.QueryExpr{}
	}
	if len(predicates) == 1 {
		return model.QueryExpr{
			Kind:      model.QueryExprPredicate,
			Predicate: predicates[0],
		}
	}
	children := make([]model.QueryExpr, 0, len(predicates))
	for _, predicate := range predicates {
		children = append(children, model.QueryExpr{
			Kind:      model.QueryExprPredicate,
			Predicate: predicate,
		})
	}
	return model.QueryExpr{Kind: model.QueryExprAnd, Children: children}
}

func (e *Engine) fieldPredicatesByID(
	ctx context.Context,
	query model.Query,
	predicates []model.QueryPredicate,
) (map[uint32][]model.QueryPredicate, error) {
	if len(predicates) == 0 {
		return nil, nil
	}
	snapshot, err := e.metadata.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]uint32, len(snapshot.Fields))
	for id, field := range snapshot.Fields {
		if field.Measurement == query.Measurement {
			ids[field.Name] = id
		}
	}
	out := make(map[uint32][]model.QueryPredicate, len(predicates))
	for _, predicate := range predicates {
		id, ok := ids[predicate.Name]
		if !ok {
			continue
		}
		out[id] = append(out[id], predicate)
	}
	return out, nil
}

func storageFieldPredicates(
	query model.Query,
	predicates []model.QueryPredicate,
) []model.QueryPredicate {
	if query.Expr.Kind != model.QueryExprNone && !queryExprAllowsFlatPredicates(query.Expr) {
		return nil
	}
	return predicates
}

func scanFields(
	fields []string,
	aggregates []model.AggregateSpec,
	predicates []model.QueryPredicate,
) []string {
	seen := make(map[string]struct{}, len(fields)+len(aggregates)+len(predicates))
	out := make([]string, 0, len(fields)+len(aggregates)+len(predicates))
	for _, field := range fields {
		out = appendUniqueField(out, seen, field)
	}
	for _, aggregate := range aggregates {
		out = appendUniqueField(out, seen, aggregate.Field)
	}
	for _, predicate := range predicates {
		out = appendUniqueField(out, seen, predicate.Name)
	}
	return out
}

func appendUniqueField(out []string, seen map[string]struct{}, field string) []string {
	if field == "" {
		return out
	}
	if _, ok := seen[field]; ok {
		return out
	}
	seen[field] = struct{}{}
	return append(out, field)
}

func (e *Engine) filterSeriesIDsByPredicates(
	ctx context.Context,
	ids []uint64,
	expr model.QueryExpr,
	predicates []model.QueryPredicate,
) ([]uint64, bool, error) {
	if expr.Kind != model.QueryExprNone && !queryExprAllowsFlatPredicates(expr) {
		if !tagOnlyExpr(expr) {
			return ids, false, nil
		}
		snapshot, err := e.metadata.Snapshot(ctx)
		if err != nil {
			return nil, false, err
		}
		return filterSeriesIDsByTagExpr(ids, snapshot.Series, expr), true, nil
	}
	if !hasNonExactTagPredicate(predicates) {
		return ids, false, nil
	}
	snapshot, err := e.metadata.Snapshot(ctx)
	if err != nil {
		return nil, false, err
	}
	return filterSeriesIDs(ids, snapshot.Series, predicates), true, nil
}

func tagOnlyExpr(expr model.QueryExpr) bool {
	switch expr.Kind {
	case model.QueryExprNone:
		return true
	case model.QueryExprPredicate:
		return isTagOrTimePredicate(expr.Predicate.Kind)
	case model.QueryExprAnd, model.QueryExprOr, model.QueryExprNot:
		for _, child := range expr.Children {
			if !tagOnlyExpr(child) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func filterSeriesIDsByTagExpr(
	ids []uint64,
	series map[uint64]catalog.Series,
	expr model.QueryExpr,
) []uint64 {
	out := ids[:0]
	for _, id := range ids {
		item, ok := series[id]
		if ok && seriesMatchesTagExpr(item.Tags, expr) {
			out = append(out, id)
		}
	}
	return out
}

func seriesMatchesTagExpr(tags map[string]string, expr model.QueryExpr) bool {
	switch expr.Kind {
	case model.QueryExprNone:
		return true
	case model.QueryExprPredicate:
		return tagPredicateMatches(tags, expr.Predicate)
	case model.QueryExprAnd:
		for _, child := range expr.Children {
			if !seriesMatchesTagExpr(tags, child) {
				return false
			}
		}
		return true
	case model.QueryExprOr:
		for _, child := range expr.Children {
			if seriesMatchesTagExpr(tags, child) {
				return true
			}
		}
		return false
	case model.QueryExprNot:
		if len(expr.Children) == 0 {
			return true
		}
		return !seriesMatchesTagExpr(tags, expr.Children[0])
	default:
		return false
	}
}

func filterSeriesIDs(
	ids []uint64,
	series map[uint64]catalog.Series,
	predicates []model.QueryPredicate,
) []uint64 {
	out := ids[:0]
	for _, id := range ids {
		if seriesMatchesTagPredicates(series[id].Tags, predicates) {
			out = append(out, id)
		}
	}
	return out
}

func seriesMatchesTagPredicates(tags map[string]string, predicates []model.QueryPredicate) bool {
	for _, predicate := range predicates {
		if !tagPredicateMatches(tags, predicate) {
			return false
		}
	}
	return true
}

func tagPredicateMatches(tags map[string]string, predicate model.QueryPredicate) bool {
	value, ok := tags[predicate.Name]
	switch predicate.Kind {
	case model.QueryPredicateTagEq:
		return ok && len(predicate.StringValues) > 0 && value == predicate.StringValues[0]
	case model.QueryPredicateTagNe:
		return !ok || len(predicate.StringValues) == 0 || value != predicate.StringValues[0]
	case model.QueryPredicateTagExists:
		return ok
	case model.QueryPredicateTagIn:
		return ok && stringIn(value, predicate.StringValues)
	default:
		return true
	}
}

func stringIn(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func hasNonExactTagPredicate(predicates []model.QueryPredicate) bool {
	for _, predicate := range predicates {
		switch predicate.Kind {
		case model.QueryPredicateTagNe,
			model.QueryPredicateTagExists,
			model.QueryPredicateTagIn:
			return true
		default:
		}
	}
	return false
}

func isTagOrTimePredicate(kind model.QueryPredicateKind) bool {
	switch kind {
	case model.QueryPredicateTimeRange,
		model.QueryPredicateTagEq,
		model.QueryPredicateTagNe,
		model.QueryPredicateTagExists,
		model.QueryPredicateTagIn:
		return true
	default:
		return false
	}
}

func queryPredicatePushdowns(
	query model.Query,
	postFilters []model.QueryPredicate,
	storageFilters []model.QueryPredicate,
) []string {
	pushdowns := make([]string, 0, 3)
	if len(postFilters) > 0 {
		pushdowns = append(pushdowns, "post_filter")
	}
	if len(storageFilters) > 0 {
		pushdowns = append(pushdowns, "field_page_stats")
	}
	if query.Order.By == model.QueryOrderByTime {
		pushdowns = append(pushdowns, "order_time")
	}
	if query.Order.By == model.QueryOrderByTime &&
		query.Order.Direction == model.QuerySortDesc &&
		query.Limit > 0 {
		pushdowns = append(pushdowns, "bounded_desc_order")
	}
	if query.Limit > 0 || query.Offset > 0 {
		pushdowns = append(pushdowns, "limit")
	}
	if len(query.Aggregates) > 0 && len(query.Group.Tags) > 0 {
		pushdowns = append(pushdowns, "group_aggregate")
	}
	return pushdowns
}

func isFieldPredicate(kind model.QueryPredicateKind) bool {
	switch kind {
	case model.QueryPredicateFieldEq,
		model.QueryPredicateFieldNe,
		model.QueryPredicateFieldGT,
		model.QueryPredicateFieldGTE,
		model.QueryPredicateFieldLT,
		model.QueryPredicateFieldLTE:
		return true
	default:
		return false
	}
}
