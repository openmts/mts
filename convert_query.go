package mts

import "github.com/openmts/mts/internal/model"

func toModelQuery(query Query) (model.Query, int64, error) {
	factor, err := timePrecisionFactor(query.Precision)
	if err != nil {
		return model.Query{}, 0, err
	}
	startTime, err := timestampToNanoseconds(query.StartTime, factor)
	if err != nil {
		return model.Query{}, 0, err
	}
	endTime, err := timestampToNanoseconds(query.EndTime, factor)
	if err != nil {
		return model.Query{}, 0, err
	}
	predicates, err := toModelQueryPredicates(query.Predicates, factor)
	if err != nil {
		return model.Query{}, 0, err
	}
	expr, err := toModelQueryExpr(query.Expr, factor)
	if err != nil {
		return model.Query{}, 0, err
	}
	return model.Query{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		Tags:            cloneStringMap(query.Tags),
		Fields:          append([]string(nil), query.Fields...),
		StartTime:       startTime,
		EndTime:         endTime,
		Predicates:      predicates,
		Expr:            expr,
		Aggregates:      toModelAggregateSpecs(query.Aggregates),
		Window:          query.Window,
		Group: model.QueryGroup{
			Tags:   append([]string(nil), query.Group.Tags...),
			Window: query.Group.Window,
		},
		Order: model.QueryOrder{
			By:        model.QueryOrderBy(query.Order.By),
			Direction: model.QuerySortDirection(query.Order.Direction),
		},
		Limit:  query.Limit,
		Offset: query.Offset,
		Budget: toModelQueryBudget(query.Budget),
	}, factor, nil
}

func toModelQueryExpr(expr QueryExpr, factor int64) (model.QueryExpr, error) {
	predicate, err := toModelQueryPredicate(expr.Predicate, factor)
	if err != nil {
		return model.QueryExpr{}, err
	}
	out := model.QueryExpr{
		Kind:      model.QueryExprKind(expr.Kind),
		Predicate: predicate,
		Children:  make([]model.QueryExpr, 0, len(expr.Children)),
	}
	for _, child := range expr.Children {
		converted, err := toModelQueryExpr(child, factor)
		if err != nil {
			return model.QueryExpr{}, err
		}
		out.Children = append(out.Children, converted)
	}
	return out, nil
}

func toModelQueryPredicates(predicates []QueryPredicate, factor int64) ([]model.QueryPredicate, error) {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		converted, err := toModelQueryPredicate(predicate, factor)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func toModelQueryPredicate(predicate QueryPredicate, factor int64) (model.QueryPredicate, error) {
	start := predicate.Start
	end := predicate.End
	if predicate.Kind == QueryPredicateTimeRange {
		convertedStart, err := timestampToNanoseconds(predicate.Start, factor)
		if err != nil {
			return model.QueryPredicate{}, err
		}
		convertedEnd, err := timestampToNanoseconds(predicate.End, factor)
		if err != nil {
			return model.QueryPredicate{}, err
		}
		start = convertedStart
		end = convertedEnd
	}
	return model.QueryPredicate{
		Kind:         model.QueryPredicateKind(predicate.Kind),
		Name:         predicate.Name,
		StringValues: append([]string(nil), predicate.StringValues...),
		Value:        toModelFieldValue(predicate.Value),
		Start:        start,
		End:          end,
	}, nil
}

func toModelAggregateSpecs(specs []AggregateSpec) []model.AggregateSpec {
	out := make([]model.AggregateSpec, len(specs))
	for index, spec := range specs {
		out[index] = model.AggregateSpec{
			Field:    spec.Field,
			Function: spec.Function,
		}
	}
	return out
}

func toModelQueryBudget(budget QueryBudget) model.QueryBudget {
	return model.QueryBudget{
		MaxShards:  budget.MaxShards,
		MaxParts:   budget.MaxParts,
		MaxSamples: budget.MaxSamples,
	}
}

func fromModelQueryBudget(budget model.QueryBudget) QueryBudget {
	return QueryBudget{
		MaxShards:  budget.MaxShards,
		MaxParts:   budget.MaxParts,
		MaxSamples: budget.MaxSamples,
	}
}

func fromModelQueryExplain(explain model.QueryExplain) QueryExplain {
	return QueryExplain{
		Database:        explain.Database,
		RetentionPolicy: explain.RetentionPolicy,
		Measurement:     explain.Measurement,
		ReadEpoch:       explain.ReadEpoch,
		TagFilters:      cloneStringMap(explain.TagFilters),
		FieldFilters:    append([]string(nil), explain.FieldFilters...),
		SeriesCount:     explain.SeriesCount,
		FieldCount:      explain.FieldCount,
		CandidateShards: explain.CandidateShards,
		MatchedShards:   explain.MatchedShards,
		SkippedShards:   explain.SkippedShards,
		Pushdowns:       append([]string(nil), explain.Pushdowns...),
		Budget:          fromModelQueryBudget(explain.Budget),
	}
}

func fromModelQueryStats(stats model.QueryStats) QueryStats {
	return QueryStats{
		CandidateShards:   stats.CandidateShards,
		ShardsScanned:     stats.ShardsScanned,
		ShardsSkipped:     stats.ShardsSkipped,
		PartsScanned:      stats.PartsScanned,
		PartsSkipped:      stats.PartsSkipped,
		IndexRowsRead:     stats.IndexRowsRead,
		IndexRowsSkipped:  stats.IndexRowsSkipped,
		TimeBlocksRead:    stats.TimeBlocksRead,
		ValueBlocksRead:   stats.ValueBlocksRead,
		ValuePagesRead:    stats.ValuePagesRead,
		ValuePagesSkipped: stats.ValuePagesSkipped,
		SamplesRead:       stats.SamplesRead,
		SamplesReturned:   stats.SamplesReturned,
		Errors:            stats.Errors,
		DurationNanos:     stats.DurationNanos,
		BudgetErrors:      stats.BudgetErrors,
		Cancellations:     stats.Cancellations,
		StartedUnixNanos:  stats.StartedUnixNanos,
		ReadEpoch:         stats.ReadEpoch,
	}
}
