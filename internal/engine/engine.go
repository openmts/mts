package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

type Engine struct {
	mu sync.Mutex

	opts                  model.Options
	deps                  Deps
	metadata              MetadataStore
	shards                map[string]*Shard
	writeSeq              atomic.Uint64
	memory                *storageMemoryLimiter
	compactionScheduler   *compactionScheduler
	queryStatsMu          sync.Mutex
	lastQueryStats        model.QueryStats
	retentionExpired      uint64
	retentionDeletedBytes uint64
	retentionDeleteErrors uint64

	compactStopOnce sync.Once
	compactStop     chan struct{}
	compactCtx      context.Context
	compactCancel   context.CancelFunc
	compactWG       sync.WaitGroup

	downsampleStopOnce sync.Once
	downsampleStop     chan struct{}
	downsampleCtx      context.Context
	downsampleCancel   context.CancelFunc
	downsampleWG       sync.WaitGroup
	downsampleRunning  map[string]struct{}
	downsampleMu       sync.Mutex
	downsampleInflight int
	downsampleSkipped  uint64
	downsampleStats    downsampleStatsRecorder
	logger             *slog.Logger
}

func Open(ctx context.Context, opts model.Options) (*Engine, error) {
	return OpenWithDeps(ctx, opts, Deps{})
}

type Deps struct {
	OpenMetadataStore func(dir string) (MetadataStore, error)
	Shard             shardDeps
}

func OpenWithDeps(_ context.Context, opts model.Options, deps Deps) (*Engine, error) {
	opts = normalizeOptions(opts)
	deps = normalizeDeps(deps, opts.Cardinality)
	if opts.Path == "" {
		return nil, fmt.Errorf("engine path is empty")
	}
	if err := prepareStorageRoot(opts.Path); err != nil {
		return nil, err
	}
	metadata, err := deps.OpenMetadataStore(catalogDir(opts.Path))
	if err != nil {
		return nil, err
	}
	eng := &Engine{
		opts:                opts,
		deps:                deps,
		metadata:            metadata,
		shards:              make(map[string]*Shard),
		memory:              newStorageMemoryLimiter(opts.StorageMemory),
		compactionScheduler: newCompactionScheduler(opts.MaxConcurrentCompaction),
		downsampleRunning:   make(map[string]struct{}),
		logger:              opts.Logger,
	}
	if err := eng.loadExistingShards(); err != nil {
		closeErr := metadata.Close()
		return nil, fmt.Errorf("load shards: %w close metadata: %v", err, closeErr)
	}
	eng.startBackgroundCompaction()
	eng.startDownsampleScheduler()
	eng.logger.Info("engine opened",
		"path", opts.Path,
		"shard_count", len(eng.shards),
	)
	return eng, nil
}

func normalizeDeps(deps Deps, limits model.CardinalityOptions) Deps {
	if deps.OpenMetadataStore == nil {
		deps.OpenMetadataStore = func(dir string) (MetadataStore, error) {
			return OpenLocalMetadataStoreWithLimits(dir, limits)
		}
	}
	deps.Shard = normalizeShardDeps(deps.Shard)
	return deps
}

func prepareStorageRoot(path string) error {
	if _, err := storagefs.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storagefs.MkdirAll(path)
		}
		return err
	}
	if err := storagefs.ValidateStrictPermissions(path); err != nil {
		if tightenEmptyDirectory(path) {
			return storagefs.ValidateStrictPermissions(path)
		}
		return err
	}
	return nil
}

func tightenEmptyDirectory(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		return false
	}
	return os.Chmod(path, storagefs.DirMode) == nil
}

func (e *Engine) Close(_ context.Context) error {
	e.stopDownsampleScheduler()
	e.stopBackgroundCompaction()
	e.mu.Lock()
	defer e.mu.Unlock()
	var errs []error
	for _, shard := range e.shards {
		if err := shard.closeForced(); err != nil {
			e.logger.Error("engine close failed",
				"shard", shardID(shard.opts.Database, shard.opts.RetentionPolicy, shard.opts.Start),
				"error", err,
			)
			errs = append(errs, err)
		}
	}
	if err := e.metadata.Close(); err != nil {
		e.logger.Error("metadata close failed", "error", err)
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	e.logger.Info("engine closed")
	return nil
}

func (e *Engine) Write(ctx context.Context, points []model.Point, opts model.WriteOptions) error {
	if len(points) == 0 {
		return nil
	}
	normalized := make([]model.Point, len(points))
	for index, point := range points {
		normalized[index] = normalizePoint(e.opts, point)
	}
	resolved, err := e.metadata.ResolvePoints(ctx, normalized)
	if err != nil {
		return err
	}
	return e.writeResolved(resolved, opts)
}

func (e *Engine) WriteTypedBatch(ctx context.Context, batch model.TypedBatch, opts model.WriteOptions) error {
	if len(batch.Timestamps) == 0 {
		return nil
	}
	normalized := normalizeTypedBatch(e.opts, batch)
	resolved, err := e.metadata.ResolveTypedBatchColumns(ctx, normalized)
	if err != nil {
		return err
	}
	return e.writeResolvedTyped(resolved, opts)
}

func (e *Engine) writeResolved(resolved []model.ResolvedPoint, opts model.WriteOptions) error {
	if len(resolved) == 0 {
		return nil
	}
	incomingSamples := resolvedSampleCount(resolved)
	incomingBytes := estimateResolvedPointsBytes(resolved)
	e.mu.Lock()
	if err := e.enforceMemoryBeforeWriteLocked(incomingSamples, incomingBytes); err != nil {
		e.mu.Unlock()
		return err
	}
	batches, err := e.groupByShardLocked(resolved)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()
	if err := writeShardBatches(batches, opts.Sync); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enforceMemoryAfterWriteLocked()
}

func (e *Engine) writeResolvedTyped(batch model.ResolvedTypedBatch, opts model.WriteOptions) error {
	if len(batch.Timestamps) == 0 {
		return nil
	}
	incomingSamples := resolvedTypedSampleCount(batch)
	incomingBytes := estimateResolvedTypedBatchBytes(batch, nil)
	e.mu.Lock()
	if err := e.enforceMemoryBeforeWriteLocked(incomingSamples, incomingBytes); err != nil {
		e.mu.Unlock()
		return err
	}
	e.assignTypedWriteSeqsLocked(&batch)
	batches, err := e.groupTypedByShardLocked(batch)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()
	if err := writeTypedShardBatches(batch, batches, opts.Sync); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enforceMemoryAfterWriteLocked()
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

func (e *Engine) RecoveryReports(_ context.Context) []RecoveryReport {
	e.mu.Lock()
	defer e.mu.Unlock()
	reports := make([]RecoveryReport, 0, len(e.shards))
	for _, shard := range e.shards {
		report := shard.RecoveryReport()
		if len(report.Issues) > 0 {
			reports = append(reports, report)
		}
	}
	return reports
}

func (e *Engine) StorageMemorySnapshot() StorageMemorySnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.memory.Snapshot(e.storageMemoryActiveLocked())
}
