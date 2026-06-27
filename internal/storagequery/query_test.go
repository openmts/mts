package storagequery_test

import (
	"context"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagequery"
)

func TestQueryCarriesSharedScanContract(t *testing.T) {
	ctx := context.Background()
	stats := &model.QueryStats{}
	query := storagequery.Query{
		Context:         ctx,
		Budget:          model.QueryBudget{MaxSamples: 10},
		Stats:           stats,
		Boundary:        model.QueryBoundaryFirst,
		SeriesIDs:       map[uint64]struct{}{1: {}},
		FieldIDs:        map[uint32]struct{}{2: {}},
		FieldPredicates: map[uint32][]model.QueryPredicate{2: {{Kind: model.QueryPredicateFieldGT}}},
		Start:           100,
		End:             200,
	}

	if query.Context != ctx {
		t.Fatalf("Context = %v, want shared context", query.Context)
	}
	if query.Stats != stats {
		t.Fatalf("Stats = %v, want shared stats", query.Stats)
	}
	if query.Budget.MaxSamples != 10 || query.Start != 100 || query.End != 200 {
		t.Fatalf("query contract lost scan fields: %#v", query)
	}
	if _, ok := query.SeriesIDs[1]; !ok {
		t.Fatalf("SeriesIDs missing expected series")
	}
	if _, ok := query.FieldIDs[2]; !ok {
		t.Fatalf("FieldIDs missing expected field")
	}
	if len(query.FieldPredicates[2]) != 1 {
		t.Fatalf("FieldPredicates = %#v, want one predicate", query.FieldPredicates)
	}
}
