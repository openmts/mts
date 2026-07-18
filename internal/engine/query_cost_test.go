package engine

import (
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEstimateQueryCostScalesWithTimeWindow(t *testing.T) {
	explain := model.QueryExplain{
		SeriesCount:     2,
		FieldCount:      3,
		MatchedShards:   1,
		CandidateShards: 1,
	}
	short := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(time.Hour),
	}, explain)
	long := estimateQueryCost(model.Query{
		StartTime: 0,
		EndTime:   int64(10 * time.Hour),
	}, explain)
	if short.EstimatedSamples <= 0 {
		t.Fatalf("short estimate = %d, want >0", short.EstimatedSamples)
	}
	if long.EstimatedSamples <= short.EstimatedSamples {
		t.Fatalf("long estimate = %d, short = %d, want long > short", long.EstimatedSamples, short.EstimatedSamples)
	}
}
