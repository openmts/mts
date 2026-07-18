package engine

import (
	"context"
	"time"

	"github.com/openmts/mts/internal/model"
)

type QueryPlan struct {
	Query                model.Query
	Explain              model.QueryExplain
	SeriesIDs            map[uint64]struct{}
	FieldIDs             map[uint32]struct{}
	FieldPredicates      map[uint32][]model.QueryPredicate
	Shards               []*Shard
	OutputFields         []string
	PostFilterPredicates []model.QueryPredicate
	Empty                bool
}

func (e *Engine) BuildQueryPlan(ctx context.Context, query model.Query) (QueryPlan, error) {
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	query = normalizeQuery(e.opts, query)
	query = normalizeStructuredQuery(query)
	outputFields := append([]string(nil), query.Fields...)
	postFilters := fieldPredicates(query.Predicates)
	storageFilters := storageFieldPredicates(query, postFilters)
	query.Fields = scanFields(query.Fields, query.Aggregates, postFilters)
	explain := model.QueryExplain{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		ReadEpoch:       time.Now().UnixNano(),
		TagFilters:      cloneTags(query.Tags),
		FieldFilters:    append([]string(nil), outputFields...),
		Budget:          query.Budget,
	}
	seriesIDs, err := e.metadata.MatchSeries(ctx, query.Measurement, query.Tags)
	if err != nil {
		return QueryPlan{}, err
	}
	var tagExprPushdown bool
	seriesIDs, tagExprPushdown, err = e.filterSeriesIDsByPredicates(ctx, seriesIDs, query.Expr, query.Predicates)
	if err != nil {
		return QueryPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.SeriesCount = len(seriesIDs)
	explain.Pushdowns = append(explain.Pushdowns, "series_id")
	explain.Pushdowns = append(explain.Pushdowns, queryPredicatePushdowns(query, postFilters, storageFilters)...)
	if tagExprPushdown {
		explain.Pushdowns = append(explain.Pushdowns, "tag_expr")
	}
	if len(seriesIDs) == 0 {
		explain.Pushdowns = append(explain.Pushdowns, "catalog_empty")
		explain.Cost = estimateQueryCost(query, explain)
		return QueryPlan{
			Query:        query,
			Explain:      explain,
			OutputFields: outputFields,
			Empty:        true,
		}, nil
	}
	fieldIDs, err := e.metadata.FieldIDs(ctx, query.Measurement, query.Fields)
	if err != nil {
		return QueryPlan{}, err
	}
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.FieldCount = len(fieldIDs)
	explain.Pushdowns = append(explain.Pushdowns, "field_id")
	fieldPredicates, err := e.fieldPredicatesByID(ctx, query, storageFilters)
	if err != nil {
		return QueryPlan{}, err
	}
	if len(query.Fields) > 0 && len(fieldIDs) == 0 {
		explain.Pushdowns = append(explain.Pushdowns, "catalog_empty")
		explain.Cost = estimateQueryCost(query, explain)
		return QueryPlan{
			Query:                query,
			Explain:              explain,
			SeriesIDs:            idSet(seriesIDs),
			FieldPredicates:      fieldPredicates,
			OutputFields:         outputFields,
			PostFilterPredicates: postFilters,
			Empty:                true,
		}, nil
	}
	shards, candidateCount := e.queryShardsWithCandidates(query)
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.CandidateShards = candidateCount
	explain.MatchedShards = len(shards)
	explain.SkippedShards = candidateCount - len(shards)
	explain.Pushdowns = append(explain.Pushdowns, "shard_time")
	explain.Cost = estimateQueryCost(query, explain)
	return QueryPlan{
		Query:                query,
		Explain:              explain,
		SeriesIDs:            idSet(seriesIDs),
		FieldIDs:             fieldIDs,
		FieldPredicates:      fieldPredicates,
		Shards:               shards,
		OutputFields:         outputFields,
		PostFilterPredicates: postFilters,
		Empty:                len(shards) == 0,
	}, nil
}

func estimateQueryCost(query model.Query, explain model.QueryExplain) model.QueryCost {
	estimate := int64(explain.SeriesCount)
	if explain.FieldCount > 0 {
		estimate *= int64(explain.FieldCount)
	}
	if explain.MatchedShards > 0 {
		estimate *= int64(explain.MatchedShards)
	}
	// 时间窗启发式：窗越长扫描潜力越高，按小时量级放大，避免仅按 series×field 低估。
	if query.EndTime > query.StartTime {
		windowHours := (query.EndTime - query.StartTime) / int64(time.Hour)
		if windowHours > 1 {
			if windowHours > 24*30 {
				windowHours = 24 * 30
			}
			estimate *= windowHours
		}
	}
	if query.Limit > 0 && estimate > int64(query.Limit) {
		estimate = int64(query.Limit)
	}
	cost := model.QueryCost{
		SeriesCount:      explain.SeriesCount,
		FieldCount:       explain.FieldCount,
		CandidateShards:  explain.CandidateShards,
		MatchedShards:    explain.MatchedShards,
		Limit:            query.Limit,
		Offset:           query.Offset,
		WindowNanos:      int64(query.Window),
		Ordered:          query.Order.By != model.QueryOrderByNone,
		Cursor:           query.Cursor != "",
		EstimatedSamples: estimate,
		PlanClass:        "scan",
	}
	if explain.SeriesCount == 0 || explain.FieldCount == 0 || explain.MatchedShards == 0 {
		cost.PlanClass = "empty"
		return cost
	}
	if query.Limit > 0 || query.Cursor != "" {
		cost.PlanClass = "bounded_scan"
	}
	if len(query.Aggregates) > 0 {
		cost.PlanClass = "aggregate"
	}
	if len(query.Group.Tags) > 0 || query.Group.Window > 0 || query.Window > 0 {
		cost.PlanClass = "group_aggregate"
	}
	return cost
}
