package engine

import (
	"context"

	"codeberg.org/mts/mts/internal/model"
)

type QueryPlan struct {
	Query     model.Query
	Explain   model.QueryExplain
	SeriesIDs map[uint64]struct{}
	FieldIDs  map[uint32]struct{}
	Shards    []*Shard
	Empty     bool
}

func (e *Engine) BuildQueryPlan(ctx context.Context, query model.Query) (QueryPlan, error) {
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	query = normalizeQuery(e.opts, query)
	explain := model.QueryExplain{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		TagFilters:      cloneTags(query.Tags),
		FieldFilters:    append([]string(nil), query.Fields...),
		Budget:          query.Budget,
	}
	seriesIDs := e.catalog.MatchSeries(query.Measurement, query.Tags)
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.SeriesCount = len(seriesIDs)
	explain.Pushdowns = append(explain.Pushdowns, "series_id")
	if len(seriesIDs) == 0 {
		explain.Pushdowns = append(explain.Pushdowns, "catalog_empty")
		return QueryPlan{Query: query, Explain: explain, Empty: true}, nil
	}
	fieldIDs := e.catalog.FieldIDs(query.Measurement, query.Fields)
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.FieldCount = len(fieldIDs)
	explain.Pushdowns = append(explain.Pushdowns, "field_id")
	if len(query.Fields) > 0 && len(fieldIDs) == 0 {
		explain.Pushdowns = append(explain.Pushdowns, "catalog_empty")
		return QueryPlan{Query: query, Explain: explain, SeriesIDs: idSet(seriesIDs), Empty: true}, nil
	}
	shards, candidateCount := e.queryShardsWithCandidates(query)
	if err := ctx.Err(); err != nil {
		return QueryPlan{}, err
	}
	explain.CandidateShards = candidateCount
	explain.MatchedShards = len(shards)
	explain.SkippedShards = candidateCount - len(shards)
	explain.Pushdowns = append(explain.Pushdowns, "shard_time")
	return QueryPlan{
		Query:     query,
		Explain:   explain,
		SeriesIDs: idSet(seriesIDs),
		FieldIDs:  fieldIDs,
		Shards:    shards,
		Empty:     len(shards) == 0,
	}, nil
}
