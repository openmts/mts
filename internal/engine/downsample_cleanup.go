package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/openmts/mts/internal/model"
)

func (e *Engine) cleanupDownsampleTarget(
	ctx context.Context,
	policy model.DownsamplePolicy,
	start int64,
	end int64,
) error {
	if start < 0 || end < 0 {
		return fmt.Errorf("downsample cleanup boundaries must be greater than or equal to zero")
	}
	if start >= end {
		return fmt.Errorf("downsample cleanup start must be before end")
	}
	target, err := e.downsampleCleanupTarget(ctx, policy, start, end)
	if err != nil || target.empty() {
		return err
	}
	return e.deleteDownsampleTargetRange(ctx, policy, start, end, target)
}

func sortedUint64Keys(values map[uint64]struct{}) []uint64 {
	out := make([]uint64, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func sortedUint32Keys(values map[uint32]struct{}) []uint32 {
	out := make([]uint32, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i] < out[j]
	})
	return out
}

type downsampleCleanupTarget struct {
	seriesIDs []uint64
	fieldIDs  []uint32
}

func (t downsampleCleanupTarget) empty() bool {
	return len(t.seriesIDs) == 0 || len(t.fieldIDs) == 0
}

func (e *Engine) downsampleCleanupTarget(
	ctx context.Context,
	policy model.DownsamplePolicy,
	start int64,
	end int64,
) (downsampleCleanupTarget, error) {
	iter, err := e.QueryColumnIterator(ctx, model.Query{
		Database:        policy.TargetDatabase,
		RetentionPolicy: policy.TargetRetention,
		Measurement:     policy.TargetMeasurement,
		Tags: map[string]string{
			policy.PolicyTagName: policy.Name,
		},
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return downsampleCleanupTarget{}, err
	}
	defer func() {
		_ = iter.Close()
	}()
	series := map[uint64]struct{}{}
	fields := map[uint32]struct{}{}
	for iter.Next() {
		column := iter.Column()
		series[column.SeriesID] = struct{}{}
		fields[column.FieldID] = struct{}{}
	}
	if err := iter.Err(); err != nil {
		return downsampleCleanupTarget{}, errors.Join(err, iter.Close())
	}
	return downsampleCleanupTarget{
		seriesIDs: sortedUint64Keys(series),
		fieldIDs:  sortedUint32Keys(fields),
	}, nil
}

func (e *Engine) deleteDownsampleTargetRange(
	ctx context.Context,
	policy model.DownsamplePolicy,
	start int64,
	end int64,
	target downsampleCleanupTarget,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	shards, _ := e.queryShardsWithCandidates(model.Query{
		Database:        policy.TargetDatabase,
		RetentionPolicy: policy.TargetRetention,
		StartTime:       start,
		EndTime:         end,
	})
	tombstone := model.Tombstone{
		SeriesIDs: target.seriesIDs,
		FieldIDs:  target.fieldIDs,
		StartTime: start,
		EndTime:   end - 1,
	}
	for _, shard := range shards {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := shard.DeleteRange(tombstone, e.opts.FlushSync); err != nil {
			return err
		}
	}
	return nil
}
