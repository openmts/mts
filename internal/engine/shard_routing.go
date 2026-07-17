package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

type shardBatch struct {
	shard  *Shard
	points []model.ResolvedPoint
}

type typedShardBatch struct {
	shard *Shard
	rows  []int
}

type shardLookupKey struct {
	database string
	policy   string
	start    int64
}

func (e *Engine) groupByShardLocked(points []model.ResolvedPoint) ([]shardBatch, error) {
	positions := make(map[*Shard]int)
	shards := make(map[shardLookupKey]*Shard, 1)
	batches := make([]shardBatch, 0, 1)
	for index := range points {
		shard, err := e.shardForPointLocked(points[index], shards)
		if err != nil {
			return nil, err
		}
		points[index].WriteSeq = e.nextWriteSeq()
		position, ok := positions[shard]
		if !ok {
			positions[shard] = len(batches)
			batch := shardBatch{shard: shard}
			if len(batches) == 0 {
				batch.points = points[:0]
			}
			batches = append(batches, batch)
			position = len(batches) - 1
		}
		batches[position].points = append(batches[position].points, points[index])
	}
	return batches, nil
}

func (e *Engine) assignTypedWriteSeqsLocked(batch *model.ResolvedTypedBatch) {
	if cap(batch.WriteSeqs) < len(batch.Timestamps) {
		batch.WriteSeqs = make([]uint64, len(batch.Timestamps))
	} else {
		batch.WriteSeqs = batch.WriteSeqs[:len(batch.Timestamps)]
	}
	for index := range batch.WriteSeqs {
		batch.WriteSeqs[index] = e.nextWriteSeq()
	}
}

func (e *Engine) groupTypedByShardLocked(
	batch model.ResolvedTypedBatch,
) ([]typedShardBatch, error) {
	positions := make(map[*Shard]int)
	shards := make(map[shardLookupKey]*Shard, 1)
	batches := make([]typedShardBatch, 0, 1)
	for row := range batch.Timestamps {
		shard, err := e.shardForTypedRowLocked(batch, row, shards)
		if err != nil {
			return nil, err
		}
		position, ok := positions[shard]
		if !ok {
			positions[shard] = len(batches)
			batches = append(batches, typedShardBatch{shard: shard})
			position = len(batches) - 1
		}
		batches[position].rows = append(batches[position].rows, row)
	}
	return batches, nil
}

func (e *Engine) shardForPointLocked(
	point model.ResolvedPoint,
	shards map[shardLookupKey]*Shard,
) (*Shard, error) {
	start := shardStart(point.Timestamp, e.opts.ShardDuration)
	key := shardLookupKey{
		database: point.Database,
		policy:   point.RetentionPolicy,
		start:    start,
	}
	if shard, ok := shards[key]; ok {
		return shard, nil
	}
	shard, err := e.shardForStartLocked(key.database, key.policy, key.start)
	if err != nil {
		return nil, err
	}
	shards[key] = shard
	return shard, nil
}

func (e *Engine) shardForTypedRowLocked(
	batch model.ResolvedTypedBatch,
	row int,
	shards map[shardLookupKey]*Shard,
) (*Shard, error) {
	start := shardStart(batch.Timestamps[row], e.opts.ShardDuration)
	key := shardLookupKey{
		database: batch.Database,
		policy:   batch.RetentionPolicy,
		start:    start,
	}
	if shard, ok := shards[key]; ok {
		return shard, nil
	}
	shard, err := e.shardForStartLocked(key.database, key.policy, key.start)
	if err != nil {
		return nil, err
	}
	shards[key] = shard
	return shard, nil
}

func (e *Engine) shardForStartLocked(database string, policy string, start int64) (*Shard, error) {
	id := shardID(database, policy, start)
	if shard, ok := e.shards[id]; ok {
		return shard, nil
	}
	dir := shardDir(e.opts.Path, database, policy, start)
	shard, maxSeq, err := OpenShard(ShardOptions{
		Dir:                dir,
		Database:           database,
		RetentionPolicy:    policy,
		Start:              start,
		End:                start + int64(e.opts.ShardDuration) - 1,
		WAL:                e.opts.WAL,
		FlushSync:          e.opts.FlushSync,
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
		Compression:        e.opts.Compression,
		Memory:             e.memory,
		scheduler:          e.compactionScheduler,
		logger:             e.logger,
		deps:               e.deps.Shard,
	})
	if err != nil {
		return nil, err
	}
	e.observeWriteSeq(maxSeq)
	e.shards[id] = shard
	return shard, nil
}

func (e *Engine) loadExistingShards() error {
	root := filepath.Join(e.opts.Path, "data")
	if _, err := storagefs.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat data root: %w", err)
	}
	if err := storagefs.ValidateStrictPermissions(root); err != nil {
		return fmt.Errorf("validate data root permissions: %w", err)
	}
	return storagefs.WalkDir(root, e.openShardDir)
}

func (e *Engine) openShardDir(path string, entry os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if !entry.IsDir() || filepath.Base(filepath.Dir(path)) != "shards" {
		return nil
	}
	if err := storagefs.ValidateStrictPermissions(path); err != nil {
		return fmt.Errorf("validate shard permissions: %w", err)
	}
	start, err := strconv.ParseInt(filepath.Base(path), 10, 64)
	if err != nil {
		return nil
	}
	policy := filepath.Base(filepath.Dir(filepath.Dir(path)))
	database := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	shard, maxSeq, err := OpenShard(ShardOptions{
		Dir:                path,
		Database:           database,
		RetentionPolicy:    policy,
		Start:              start,
		End:                start + int64(e.opts.ShardDuration) - 1,
		WAL:                e.opts.WAL,
		FlushSync:          e.opts.FlushSync,
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
		Compression:        e.opts.Compression,
		Memory:             e.memory,
		scheduler:          e.compactionScheduler,
		logger:             e.logger,
		deps:               e.deps.Shard,
	})
	if err != nil {
		return err
	}
	e.shards[shardID(database, policy, start)] = shard
	e.observeWriteSeq(maxSeq)
	return nil
}
