package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"codeberg.org/mts/mts/internal/catalog"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/observability"
	"codeberg.org/mts/mts/internal/sstable"
	"codeberg.org/mts/mts/internal/storagefs"
	"codeberg.org/mts/mts/internal/wal"
)

type Engine struct {
	mu sync.Mutex

	opts                model.Options
	catalog             *catalog.Catalog
	shards              map[string]*Shard
	writeSeq            uint64
	memory              *storageMemoryLimiter
	compactionScheduler *compactionScheduler
	queryStatsMu        sync.Mutex
	lastQueryStats      model.QueryStats
	retentionExpired    uint64

	compactStopOnce sync.Once
	compactStop     chan struct{}
	compactWG       sync.WaitGroup
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
	if err := prepareStorageRoot(opts.Path); err != nil {
		return nil, err
	}
	cat, err := catalog.Open(catalogDir(opts.Path))
	if err != nil {
		return nil, err
	}
	eng := &Engine{
		opts:                opts,
		catalog:             cat,
		shards:              make(map[string]*Shard),
		memory:              newStorageMemoryLimiter(opts.StorageMemory),
		compactionScheduler: newCompactionScheduler(),
	}
	if err := eng.loadExistingShards(); err != nil {
		closeErr := cat.Close()
		return nil, fmt.Errorf("load shards: %w close catalog: %v", err, closeErr)
	}
	eng.startBackgroundCompaction()
	return eng, nil
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
	e.stopBackgroundCompaction()
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
	incomingSamples := resolvedSampleCount(resolved)
	incomingBytes := estimateResolvedPointsBytes(resolved)
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.enforceMemoryBeforeWriteLocked(incomingSamples, incomingBytes); err != nil {
		return err
	}
	batches, err := e.groupByShardLocked(resolved)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		if err := batch.shard.WriteBatch(batch.points, opts.Sync); err != nil {
			return err
		}
	}
	if err := e.enforceMemoryAfterWriteLocked(); err != nil {
		return err
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

func resolvedSampleCount(points []model.ResolvedPoint) int {
	total := 0
	for _, point := range points {
		total += len(point.Fields)
	}
	return total
}

func estimateResolvedPointsBytes(points []model.ResolvedPoint) int64 {
	var total int64
	for _, point := range points {
		total += int64(32 + len(point.Measurement))
		for key, value := range point.Tags {
			total += int64(len(key) + len(value) + 32)
		}
		for _, field := range point.Fields {
			total += estimateResolvedFieldBytes(field)
		}
	}
	return total
}

func estimateWALFrameBytes(points []model.ResolvedPoint) int64 {
	const frameAndRecordOverhead = int64(64)
	return estimateResolvedPointsBytes(points) + frameAndRecordOverhead + int64(len(points))*16
}

func estimateResolvedFieldBytes(field model.ResolvedField) int64 {
	const sampleBaseBytes = int64(32)
	switch field.Type {
	case model.FieldFloat64, model.FieldInt64:
		return sampleBaseBytes + 8
	case model.FieldString:
		return sampleBaseBytes + 16 + int64(len(field.Value.String))
	case model.FieldBool:
		return sampleBaseBytes + 1
	default:
		return sampleBaseBytes
	}
}

func (e *Engine) enforceMemoryBeforeWriteLocked(incomingSamples int, incomingBytes int64) error {
	memory := e.opts.StorageMemory
	if memory.HardSampleLimit > 0 && incomingSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: incoming=%d limit=%d",
			incomingSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && incomingBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, incomingBytes, memory.HardBytesLimit)
	}
	currentSamples := e.totalMemSamplesLocked()
	currentBytes := e.totalMemBytesLocked()
	if shouldFlushBeforeWrite(memory, currentSamples, incomingSamples, currentBytes, incomingBytes) {
		if err := e.flushAllShardsLocked(); err != nil {
			return err
		}
		e.memory.RecordFlushTriggered()
		currentSamples = e.totalMemSamplesLocked()
		currentBytes = e.totalMemBytesLocked()
	}
	if memory.HardSampleLimit > 0 && currentSamples+incomingSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: current=%d incoming=%d limit=%d",
			currentSamples,
			incomingSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && currentBytes+incomingBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, currentBytes+incomingBytes, memory.HardBytesLimit)
	}
	return nil
}

func (e *Engine) enforceMemoryAfterWriteLocked() error {
	memory := e.opts.StorageMemory
	currentSamples := e.totalMemSamplesLocked()
	currentBytes := e.totalMemBytesLocked()
	if shouldFlushAfterWrite(memory, currentSamples, currentBytes) {
		if err := e.flushAllShardsLocked(); err != nil {
			return err
		}
		e.memory.RecordFlushTriggered()
		currentSamples = e.totalMemSamplesLocked()
		currentBytes = e.totalMemBytesLocked()
	}
	if memory.HardSampleLimit > 0 && currentSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: current=%d limit=%d",
			currentSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && currentBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, currentBytes, memory.HardBytesLimit)
	}
	return nil
}

func shouldFlushBeforeWrite(
	memory model.StorageMemoryOptions,
	currentSamples int,
	incomingSamples int,
	currentBytes int64,
	incomingBytes int64,
) bool {
	if incomingSamples == 0 && incomingBytes == 0 {
		return false
	}
	if memory.HardSampleLimit > 0 && currentSamples+incomingSamples > memory.HardSampleLimit {
		return true
	}
	if memory.HardBytesLimit > 0 && currentBytes+incomingBytes > memory.HardBytesLimit {
		return true
	}
	if memory.SoftSampleLimit > 0 && currentSamples+incomingSamples >= memory.SoftSampleLimit && currentSamples > 0 {
		return true
	}
	return memory.SoftBytesLimit > 0 && currentBytes+incomingBytes >= memory.SoftBytesLimit && currentBytes > 0
}

func shouldFlushAfterWrite(memory model.StorageMemoryOptions, currentSamples int, currentBytes int64) bool {
	if memory.SoftSampleLimit > 0 && currentSamples >= memory.SoftSampleLimit {
		return true
	}
	return memory.SoftBytesLimit > 0 && currentBytes >= memory.SoftBytesLimit
}

func (e *Engine) flushAllShardsLocked() error {
	for _, shard := range e.shards {
		if err := shard.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) totalMemSamplesLocked() int {
	total := 0
	for _, shard := range e.shards {
		total += shard.ApproxMemorySamples()
	}
	return total
}

func (e *Engine) totalMemBytesLocked() int64 {
	return e.storageMemoryActiveLocked().total()
}

func (e *Engine) storageMemoryActiveLocked() storageMemoryActive {
	var active storageMemoryActive
	for _, shard := range e.shards {
		memBytes := shard.ApproxMemTableMemoryBytes()
		walBytes := shard.ApproxWALMemoryBytes()
		active.MemTableBytes += memBytes
		active.WALBytes += walBytes
	}
	return active
}

func (e *Engine) MaintenanceErrors(_ context.Context) []error {
	e.mu.Lock()
	defer e.mu.Unlock()
	errs := make([]error, 0)
	for _, shard := range e.shards {
		if shard.maintenanceErr != nil {
			errs = append(errs, shard.maintenanceErr)
		}
	}
	return errs
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

func (e *Engine) MetricsSnapshot() []observability.Metric {
	snapshot := e.StorageMemorySnapshot()
	stats := e.CompactionStatsSnapshot()
	production := e.productionMetricsSnapshot()
	queryStats := e.QueryStatsSnapshot()
	registry := observability.NewRegistry()
	recordStorageMemoryMetrics(registry, snapshot)
	recordCompactionMetrics(registry, stats)
	recordWALMetrics(registry, e.walMetricsSnapshot())
	recordMemTableMetrics(registry, production)
	recordSSTableMetrics(registry, production)
	recordQueryMetrics(registry, queryStats)
	recordRetentionMetrics(registry, production)
	recordRecoveryMetrics(registry, production)
	recordRuntimeMetrics(registry, runtimeMetricsSnapshot())
	return registry.Snapshot()
}

type productionMetrics struct {
	MemTableSamples       int
	MemTableBytes         int64
	SSTableParts          int
	SSTableRows           int
	SSTableSeries         int
	SSTableBlocks         int
	SSTableMaxLevel       int
	SSTableLevel0Parts    int
	SSTableMaxWriteSeq    uint64
	RetentionExpiredParts uint64
	RecoveryIssues        int
	RecoveryErrors        int
	RecoveryFatalErrors   int
}

func (e *Engine) productionMetricsSnapshot() productionMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := productionMetrics{RetentionExpiredParts: e.retentionExpired}
	for _, shard := range e.shards {
		out.MemTableSamples += shard.mem.SampleCount()
		out.MemTableBytes += shard.mem.ApproxMemoryBytes()
		out.RecoveryIssues += len(shard.recoveryReport.Issues)
		out.RecoveryErrors += recoveryErrorCount(shard.recoveryReport)
		out.RecoveryFatalErrors += recoveryFatalCount(shard.recoveryReport)
		for _, part := range shard.manifest.Parts {
			mergePartMetrics(&out, part)
		}
	}
	return out
}

func mergePartMetrics(out *productionMetrics, part sstable.PartMeta) {
	out.SSTableParts++
	out.SSTableRows += part.RowsCount
	out.SSTableSeries += part.SeriesCount
	out.SSTableBlocks += part.BlockCount
	if part.Level > out.SSTableMaxLevel {
		out.SSTableMaxLevel = part.Level
	}
	if part.Level == 0 {
		out.SSTableLevel0Parts++
	}
	if part.MaxWriteSeq > out.SSTableMaxWriteSeq {
		out.SSTableMaxWriteSeq = part.MaxWriteSeq
	}
}

func recoveryErrorCount(report RecoveryReport) int {
	total := 0
	for _, issue := range report.Issues {
		if issue.Err != nil {
			total++
		}
	}
	return total
}

func recoveryFatalCount(report RecoveryReport) int {
	total := 0
	for _, issue := range report.Issues {
		if issue.Fatal {
			total++
		}
	}
	return total
}

func (e *Engine) walMetricsSnapshot() wal.Metrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out wal.Metrics
	for _, shard := range e.shards {
		if shard.wal == nil {
			continue
		}
		provider, ok := shard.wal.(walMetricsProvider)
		if !ok {
			continue
		}
		out = mergeWALMetrics(out, provider.MetricsSnapshot())
	}
	return out
}

func mergeWALMetrics(left wal.Metrics, right wal.Metrics) wal.Metrics {
	left.AppendRecords += right.AppendRecords
	left.AppendErrors += right.AppendErrors
	left.AppendLatencyNanos += right.AppendLatencyNanos
	left.SyncCount += right.SyncCount
	left.SyncErrors += right.SyncErrors
	left.SyncLatencyNanos += right.SyncLatencyNanos
	left.CheckpointCount += right.CheckpointCount
	left.CheckpointErrors += right.CheckpointErrors
	left.ReplayRecords += right.ReplayRecords
	left.ReplayErrors += right.ReplayErrors
	left.ReplayLatencyNanos += right.ReplayLatencyNanos
	left.SegmentCount += right.SegmentCount
	left.PendingRecords += right.PendingRecords
	left.PendingBytes += right.PendingBytes
	return left
}

func (e *Engine) CompactionStatsSnapshot() CompactionStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out CompactionStats
	for _, shard := range e.shards {
		out = mergeCompactionStats(out, shard.CompactionStatsSnapshot())
		backlog, err := shard.compactionBacklogSnapshot(e.opts.Compaction)
		if err != nil {
			out.LastError = err.Error()
			continue
		}
		out.Backlog += backlog.PendingPlans
		out.OverlapCount += backlog.OverlapCount
		if backlog.MaxScore > out.MaxScore {
			out.MaxScore = backlog.MaxScore
		}
	}
	out = mergeCompactionSchedulerStats(out, e.compactionScheduler.snapshotCopy())
	return out
}

type HealthSnapshot struct {
	Healthy bool
	Ready   bool
	Reasons []string
}

func (e *Engine) HealthSnapshot() HealthSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	health := HealthSnapshot{Healthy: true, Ready: true, Reasons: []string{}}
	for id, shard := range e.shards {
		backlog, err := shard.compactionBacklogSnapshot(e.opts.Compaction)
		if err != nil {
			health.Healthy = false
			health.Ready = false
			health.Reasons = append(health.Reasons, "compaction backlog check failed: "+err.Error())
			continue
		}
		if backlog.Degraded {
			health.Healthy = false
			health.Ready = false
			health.Reasons = append(health.Reasons, "compaction degraded on shard "+id)
		}
		if shard.maintenanceErr != nil {
			health.Healthy = false
			health.Reasons = append(health.Reasons, "maintenance error on shard "+id+": "+shard.maintenanceErr.Error())
		}
	}
	return health
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
		Compression:        e.opts.Compression,
		Memory:             e.memory,
		scheduler:          e.compactionScheduler,
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
		MemTableMaxSamples: e.opts.MemTableMaxSamples,
		Compaction:         e.opts.Compaction,
		Compression:        e.opts.Compression,
		Memory:             e.memory,
		scheduler:          e.compactionScheduler,
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
