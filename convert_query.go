package mts

import "github.com/openmts/mts/internal/model"

func toModelQuery(query Query) model.Query {
	return model.Query{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		Tags:            cloneStringMap(query.Tags),
		Fields:          append([]string(nil), query.Fields...),
		StartTime:       query.StartTime,
		EndTime:         query.EndTime,
		Predicates:      toModelQueryPredicates(query.Predicates),
		Expr:            toModelQueryExpr(query.Expr),
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
	}
}

func toModelQueryExpr(expr QueryExpr) model.QueryExpr {
	out := model.QueryExpr{
		Kind:      model.QueryExprKind(expr.Kind),
		Predicate: toModelQueryPredicate(expr.Predicate),
		Children:  make([]model.QueryExpr, 0, len(expr.Children)),
	}
	for _, child := range expr.Children {
		out.Children = append(out.Children, toModelQueryExpr(child))
	}
	return out
}

func toModelQueryPredicates(predicates []QueryPredicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		out = append(out, toModelQueryPredicate(predicate))
	}
	return out
}

func toModelQueryPredicate(predicate QueryPredicate) model.QueryPredicate {
	return model.QueryPredicate{
		Kind:         model.QueryPredicateKind(predicate.Kind),
		Name:         predicate.Name,
		StringValues: append([]string(nil), predicate.StringValues...),
		Value:        toModelFieldValue(predicate.Value),
		Start:        predicate.Start,
		End:          predicate.End,
	}
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
