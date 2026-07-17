package engine

import (
	"context"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

// Delete 按时间范围对匹配 shard 写入 tombstone。
// Tags 为空时删除 measurement 在时间范围内的全部 series；非空时仅删除匹配 series。
func (e *Engine) Delete(ctx context.Context, req model.DeleteRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateDeleteRequest(req); err != nil {
		return err
	}
	req = normalizeDeleteRequest(e.opts, req)

	var seriesIDs []uint64
	if len(req.Tags) > 0 || req.Measurement != "" {
		ids, err := e.metadata.MatchSeries(ctx, req.Measurement, req.Tags)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		seriesIDs = ids
	}

	tombstone := model.Tombstone{
		SeriesIDs: seriesIDs,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		WriteSeq:  e.nextWriteSeq(),
	}

	e.mu.Lock()
	shards := make([]*Shard, 0, len(e.shards))
	for _, shard := range e.shards {
		if !shardMatches(shard, model.Query{
			Database:        req.Database,
			RetentionPolicy: req.RetentionPolicy,
			StartTime:       req.StartTime,
			EndTime:         req.EndTime,
		}) {
			continue
		}
		shards = append(shards, shard)
	}
	e.mu.Unlock()

	for _, shard := range shards {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := shard.DeleteRange(tombstone, e.opts.FlushSync); err != nil {
			return fmt.Errorf("delete range on shard %s: %w",
				shardID(shard.opts.Database, shard.opts.RetentionPolicy, shard.opts.Start), err)
		}
	}
	return nil
}

func validateDeleteRequest(req model.DeleteRequest) error {
	if req.EndTime < req.StartTime {
		return fmt.Errorf("delete end time before start time")
	}
	return nil
}

func normalizeDeleteRequest(opts model.Options, req model.DeleteRequest) model.DeleteRequest {
	if req.Database == "" {
		req.Database = opts.DefaultDatabase
	}
	if req.RetentionPolicy == "" {
		req.RetentionPolicy = opts.DefaultRetentionPolicy
	}
	return req
}
