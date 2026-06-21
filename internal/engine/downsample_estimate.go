package engine

import (
	"context"
	"errors"

	"github.com/openmts/mts/internal/model"
)

type downsampleCostEstimate struct {
	groups  int
	samples int
}

func (e *Engine) estimateDownsampleCost(
	ctx context.Context,
	policy model.DownsamplePolicy,
	start int64,
	end int64,
) (downsampleCostEstimate, error) {
	iter, err := e.QueryColumnIterator(ctx, downsampleSourceQuery(policy, start, end))
	if err != nil {
		return downsampleCostEstimate{}, err
	}
	defer func() {
		_ = iter.Close()
	}()
	groups := map[string]struct{}{}
	samples := 0
	for iter.Next() {
		column := iter.Column()
		count := downsampleSamplesInRange(column, start, end)
		if count == 0 {
			continue
		}
		samples += count
		groups[estimateDownsampleGroupKey(column.Tags, policy.GroupByTags)] = struct{}{}
	}
	if err := iter.Err(); err != nil {
		return downsampleCostEstimate{}, errors.Join(err, iter.Close())
	}
	return downsampleCostEstimate{groups: len(groups), samples: samples}, nil
}

func downsampleSamplesInRange(column model.ColumnSeries, start int64, end int64) int {
	count := 0
	for _, timestamp := range column.Timestamps {
		if timestamp >= start && timestamp < end {
			count++
		}
	}
	return count
}
