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
	for _, point := range points {
		if err := e.writePoint(point, opts); err != nil {
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

func (e *Engine) writePoint(point model.Point, opts model.WriteOptions) error {
	point = normalizePoint(e.opts, point)
	resolved, err := e.catalog.ResolvePoint(point)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.writeSeq++
	resolved.WriteSeq = e.writeSeq
	shard, err := e.shardForLocked(resolved.Database, resolved.RetentionPolicy, resolved.Timestamp)
	if err != nil {
		return err
	}
	if err := shard.Write(resolved, opts.Sync); err != nil {
		return err
	}
	if resolved.WriteSeq > e.writeSeq {
		e.writeSeq = resolved.WriteSeq
	}
	return nil
}

func (e *Engine) shardForLocked(database string, policy string, timestamp int64) (*Shard, error) {
	start := shardStart(timestamp, e.opts.ShardDuration)
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
