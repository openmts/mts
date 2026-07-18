package engine

import (
	"context"
	"os"
	"time"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/observability"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/wal"
)

func (e *Engine) MetricsSnapshot() []observability.Metric {
	snapshot := e.StorageMemorySnapshot()
	stats := e.CompactionStatsSnapshot()
	production := e.productionMetricsSnapshot()
	queryStats := e.QueryStatsSnapshot()
	downsampleStats := e.DownsampleStatsSnapshot()
	downsampleStatuses, downsampleStatusErr := e.DownsamplePolicyStatuses(
		context.Background(),
		time.Duration(time.Now().UnixNano()),
	)
	registry := observability.NewRegistry()
	recordStorageMemoryMetrics(registry, snapshot)
	recordCompactionMetrics(registry, stats)
	recordTombstoneMetrics(registry, production)
	recordWALMetrics(registry, e.walMetricsSnapshot())
	recordMemTableMetrics(registry, production)
	recordSSTableMetrics(registry, production)
	recordQueryMetrics(registry, queryStats)
	recordRetentionMetrics(registry, production)
	recordDownsampleMetrics(registry, downsampleStats)
	if downsampleStatusErr != nil {
		registry.AddCounter(
			"mts_downsample_status_collection_errors_total",
			"Downsample status collection errors.",
			1,
		)
	}
	recordDownsamplePolicyMetrics(registry, downsampleStatuses)
	recordMaintenanceMetrics(registry, e.MaintenanceStatsSnapshot())
	recordRecoveryMetrics(registry, production)
	recordRuntimeMetrics(registry, runtimeMetricsSnapshot())
	return registry.Snapshot()
}

func (e *Engine) DownsampleStatsSnapshot() model.DownsampleStats {
	return e.downsampleStats.snapshot()
}

type productionMetrics struct {
	MemTableSamples         int
	MemTableBytes           int64
	MemTableSeries          int
	MemTableFields          int
	MemTableColumns         int
	MemTableFlushTriggered  uint64
	MemTableOutOfOrder      uint64
	MemTableDuplicates      uint64
	MemTableAppended        uint64
	SSTableParts            int
	SSTableRows             int
	SSTableSeries           int
	SSTableBlocks           int
	SSTableMaxLevel         int
	SSTableLevel0Parts      int
	SSTableMaxWriteSeq      uint64
	SSTableDataBytes        int64
	SSTableIndexBytes       int64
	SSTableTotalBytes       int64
	SSTableCompressionRatio float64
	RetentionExpiredParts   uint64
	RetentionDeletedBytes   uint64
	RetentionDeleteErrors   uint64
	RecoveryIssues          int
	RecoveryErrors          int
	RecoveryFatalErrors     int
	TombstoneCount          int
}

func (e *Engine) productionMetricsSnapshot() productionMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := productionMetrics{
		RetentionExpiredParts: e.retentionExpired,
		RetentionDeletedBytes: e.retentionDeletedBytes,
		RetentionDeleteErrors: e.retentionDeleteErrors,
	}
	if e.memory != nil {
		e.memory.mu.Lock()
		out.MemTableFlushTriggered = e.memory.flushTriggered
		e.memory.mu.Unlock()
	}
	for _, shard := range e.shards {
		mergeMemTableMetrics(&out, shard.mem)
		out.RecoveryIssues += len(shard.recoveryReport.Issues)
		out.RecoveryErrors += recoveryErrorCount(shard.recoveryReport)
		out.RecoveryFatalErrors += recoveryFatalCount(shard.recoveryReport)
		out.TombstoneCount += len(shard.tombstones)
		for _, part := range shard.manifest.Parts {
			mergePartMetrics(&out, part)
		}
	}
	out.SSTableCompressionRatio = sstableCompressionRatio(out.SSTableRows, out.SSTableDataBytes)
	return out
}

type memStatsProvider interface {
	StatsSnapshot() memtable.Stats
}

func mergeMemTableMetrics(out *productionMetrics, store memStore) {
	if store == nil {
		return
	}
	provider, ok := store.(memStatsProvider)
	if !ok {
		out.MemTableSamples += store.SampleCount()
		out.MemTableBytes += store.ApproxMemoryBytes()
		return
	}
	stats := provider.StatsSnapshot()
	out.MemTableSamples += stats.Samples
	out.MemTableBytes += stats.Bytes
	out.MemTableOutOfOrder += stats.OutOfOrderSamples
	out.MemTableDuplicates += stats.DuplicateSamples
	out.MemTableAppended += stats.AppendedSamples
	out.MemTableSeries += stats.Series
	out.MemTableFields += stats.Fields
	out.MemTableColumns += stats.Columns
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
	sizes := sstableFileSizes(part.Path)
	out.SSTableDataBytes += sizes.dataBytes
	out.SSTableIndexBytes += sizes.indexBytes
	out.SSTableTotalBytes += sizes.totalBytes
}

type sstableSizeMetrics struct {
	dataBytes  int64
	indexBytes int64
	totalBytes int64
}

func sstableFileSizes(path string) sstableSizeMetrics {
	entries, err := os.ReadDir(path)
	if err != nil {
		return sstableSizeMetrics{}
	}
	var out sstableSizeMetrics
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		out.totalBytes += size
		if isSSTableDataFile(entry.Name()) {
			out.dataBytes += size
			continue
		}
		out.indexBytes += size
	}
	return out
}

func isSSTableDataFile(name string) bool {
	return name == "timestamps.bin" || name == "values.bin" || name == "strings.bin"
}

func sstableCompressionRatio(rows int, dataBytes int64) float64 {
	if rows <= 0 || dataBytes <= 0 {
		return 0
	}
	const estimatedWideRowBytes = 80
	return float64(rows*estimatedWideRowBytes) / float64(dataBytes)
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
