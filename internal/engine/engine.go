package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"codeberg.org/mts/mts/internal/catalog"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

type Engine struct {
	mu sync.Mutex

	opts     model.Options
	catalog  *catalog.Catalog
	shards   map[string]*Shard
	writeSeq uint64
}

type shardBatch struct {
	shard  *Shard
	points []model.ResolvedPoint
}

type shardLookupKey struct {
	database string
	policy   string
	start    int64
}

func Open(_ context.Context, opts model.Options) (*Engine, error) {
	opts = normalizeOptions(opts)
	if opts.Path == "" {
		return nil, fmt.Errorf("engine path is empty")
	}
	if err := storagefs.MkdirAll(opts.Path); err != nil {
		return nil, err
	}
	cat, err := catalog.Open(catalogDir(opts.Path))
	if err != nil {
		return nil, err
	}
	eng := &Engine{
		opts:    opts,
		catalog: cat,
		shards:  make(map[string]*Shard),
	}
	if err := eng.loadExistingShards(); err != nil {
		closeErr := cat.Close()
		return nil, fmt.Errorf("load shards: %w close catalog: %v", err, closeErr)
	}
	return eng, nil
}

func (e *Engine) Close(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, shard := range e.shards {
		if err := shard.Close(); err != nil {
			return err
		}
	}
	if err := e.catalog.Close(); err != nil {
		return err
	}
	return nil
}

func (e *Engine) Write(_ context.Context, points []model.Point, opts model.WriteOptions) error {
	if len(points) == 0 {
		return nil
	}
	normalized := make([]model.Point, len(points))
	for index, point := range points {
		normalized[index] = normalizePoint(e.opts, point)
	}
	resolved, err := e.catalog.ResolvePoints(normalized)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	batches, err := e.groupByShardLocked(resolved)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := batch.shard.WriteBatch(batch.points, opts.Sync); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Flush(_ context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, shard := range e.shards {
		if err := shard.Flush(); err != nil {
			return err
		}
	}
	return nil
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
		e.writeSeq++
		points[index].WriteSeq = e.writeSeq
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
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
	})
	if err != nil {
		return nil, err
	}
	if maxSeq > e.writeSeq {
		e.writeSeq = maxSeq
	}
	e.shards[id] = shard
	return shard, nil
}

func (e *Engine) loadExistingShards() error {
	root := filepath.Join(e.opts.Path, "data")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat data root: %w", err)
	}
	return filepath.WalkDir(root, e.openShardDir)
}

func (e *Engine) openShardDir(path string, entry os.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if !entry.IsDir() || filepath.Base(filepath.Dir(path)) != "shards" {
		return nil
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
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
	})
	if err != nil {
		return err
	}
	e.shards[shardID(database, policy, start)] = shard
	if maxSeq > e.writeSeq {
		e.writeSeq = maxSeq
	}
	return nil
}
