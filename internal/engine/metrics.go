package engine

import (
	"os"
	"runtime"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/observability"
	"github.com/openmts/mts/internal/wal"
)

type runtimeMetrics struct {
	HeapAllocBytes    uint64
	HeapInuseBytes    uint64
	GCTotal           uint32
	GCPauseTotalNanos uint64
	Goroutines        int
	FDOpen            int
}

func recordStorageMemoryMetrics(registry *observability.Registry, snapshot StorageMemorySnapshot) {
	registry.SetGauge("mts_storage_memory_current_bytes", "Estimated storage memory bytes in use.", float64(snapshot.CurrentBytes))
	registry.SetGauge("mts_storage_memory_peak_bytes", "Peak estimated storage memory bytes.", float64(snapshot.PeakBytes))
	registry.SetGauge("mts_storage_memory_active_bytes", "Estimated active storage memory bytes.", float64(snapshot.ActiveBytes))
	registry.SetGauge("mts_storage_memory_memtable_bytes", "Estimated MemTable memory bytes.", float64(snapshot.MemTableBytes))
	registry.SetGauge("mts_storage_memory_wal_bytes", "Estimated WAL pending memory bytes.", float64(snapshot.WALBytes))
	registry.SetGauge("mts_storage_memory_reservation_bytes", "Estimated reserved temporary storage memory bytes.", float64(snapshot.ReservationBytes))
	registry.SetGauge("mts_storage_memory_write_bytes", "Estimated write-path temporary storage memory bytes.", float64(snapshot.WriteBytes))
	registry.SetGauge("mts_storage_memory_flush_bytes", "Estimated flush temporary storage memory bytes.", float64(snapshot.FlushBytes))
	registry.SetGauge("mts_storage_memory_query_bytes", "Estimated query temporary storage memory bytes.", float64(snapshot.QueryBytes))
	registry.SetGauge("mts_storage_memory_compaction_bytes", "Estimated compaction temporary storage memory bytes.", float64(snapshot.CompactionBytes))
	registry.SetGauge("mts_storage_memory_compression_bytes", "Estimated compression temporary storage memory bytes.", float64(snapshot.CompressionBytes))
	registry.SetGauge("mts_storage_memory_soft_limit_bytes", "Configured storage memory soft limit bytes.", float64(snapshot.SoftBytesLimit))
	registry.SetGauge("mts_storage_memory_hard_limit_bytes", "Configured storage memory hard limit bytes.", float64(snapshot.HardBytesLimit))
	registry.SetGauge("mts_storage_memory_runtime_heap_alloc_bytes", "Go runtime heap allocation bytes.", float64(snapshot.RuntimeHeapAllocBytes))
	registry.SetGauge("mts_storage_memory_runtime_rss_bytes", "Process RSS bytes.", float64(snapshot.RuntimeRSSBytes))
	registry.SetGauge("mts_storage_memory_runtime_gap_bytes", "Process RSS minus estimated storage memory bytes.", float64(snapshot.RuntimeGapBytes))
	registry.AddCounter("mts_storage_memory_rejected_writes_total", "Storage writes rejected by memory budget.", float64(snapshot.RejectedWrites))
	registry.AddCounter("mts_storage_memory_rejected_reservations_total", "Temporary storage reservations rejected by memory budget.", float64(snapshot.RejectedReservations))
	registry.AddCounter("mts_storage_memory_flush_triggered_total", "Flushes triggered by storage memory budget.", float64(snapshot.FlushTriggered))
}

func recordCompactionMetrics(registry *observability.Registry, stats CompactionStats) {
	registry.SetGauge("mts_compaction_active", "Active compaction tasks.", float64(stats.Active))
	registry.SetGauge("mts_compaction_backlog", "Pending compaction plans.", float64(stats.Backlog))
	registry.SetGauge("mts_compaction_overlap_count", "Detected compaction level overlaps.", float64(stats.OverlapCount))
	registry.SetGauge("mts_compaction_last_score", "Last or highest observed compaction score.", stats.MaxScore)
	registry.SetGauge("mts_compaction_last_duration_seconds", "Last compaction duration in seconds.", stats.LastDuration.Seconds())
	registry.AddCounter("mts_compaction_total", "Total compaction attempts.", float64(stats.Total))
	registry.AddCounter("mts_compaction_success_total", "Successful compaction attempts.", float64(stats.Success))
	registry.AddCounter("mts_compaction_errors_total", "Failed compaction attempts.", float64(stats.Failure))
	registry.AddCounter("mts_compaction_input_bytes_total", "Compaction input bytes.", float64(stats.InputBytes))
	registry.AddCounter("mts_compaction_output_bytes_total", "Compaction output bytes.", float64(stats.OutputBytes))
	registry.AddCounter("mts_compaction_dropped_rows_total", "Rows dropped by compaction merge or tombstones.", float64(stats.DroppedRows))
	registry.AddCounter("mts_compaction_skipped_total", "Skipped compaction scheduler attempts.", float64(stats.Skipped))
	registry.AddCounter("mts_compaction_safe_delete_parts_total", "Input parts safely removed after manifest commit.", float64(stats.SafeDeleteParts))
}

func recordTombstoneMetrics(registry *observability.Registry, snapshot productionMetrics) {
	registry.SetGauge("mts_tombstones_pending", "Logical tombstones pending physical reclaim via compaction.", float64(snapshot.TombstoneCount))
}

func recordWALMetrics(registry *observability.Registry, snapshot wal.Metrics) {
	registry.AddCounter("mts_wal_append_records_total", "WAL records appended.", float64(snapshot.AppendRecords))
	registry.AddCounter("mts_wal_append_errors_total", "WAL append errors.", float64(snapshot.AppendErrors))
	registry.ObserveHistogram("mts_wal_append_latency_seconds", "Cumulative WAL append latency.", nanosToSeconds(snapshot.AppendLatencyNanos))
	registry.AddCounter("mts_wal_sync_total", "WAL fsync operations.", float64(snapshot.SyncCount))
	registry.AddCounter("mts_wal_sync_errors_total", "WAL fsync errors.", float64(snapshot.SyncErrors))
	registry.ObserveHistogram("mts_wal_sync_latency_seconds", "Cumulative WAL fsync latency.", nanosToSeconds(snapshot.SyncLatencyNanos))
	registry.AddCounter("mts_wal_checkpoint_total", "WAL checkpoint operations.", float64(snapshot.CheckpointCount))
	registry.AddCounter("mts_wal_checkpoint_errors_total", "WAL checkpoint errors.", float64(snapshot.CheckpointErrors))
	registry.AddCounter("mts_wal_replay_records_total", "WAL records replayed.", float64(snapshot.ReplayRecords))
	registry.AddCounter("mts_wal_replay_errors_total", "WAL replay errors.", float64(snapshot.ReplayErrors))
	registry.ObserveHistogram("mts_wal_replay_latency_seconds", "Cumulative WAL replay latency.", nanosToSeconds(snapshot.ReplayLatencyNanos))
	registry.SetGauge("mts_wal_segments", "WAL segment count.", float64(snapshot.SegmentCount))
	registry.SetGauge("mts_wal_pending_records", "Pending WAL records before fsync.", float64(snapshot.PendingRecords))
	registry.SetGauge("mts_wal_pending_bytes", "Pending WAL bytes before fsync.", float64(snapshot.PendingBytes))
}

func recordMemTableMetrics(registry *observability.Registry, snapshot productionMetrics) {
	registry.SetGauge("mts_memtable_samples", "MemTable samples currently buffered.", float64(snapshot.MemTableSamples))
	registry.SetGauge("mts_memtable_estimated_bytes", "Estimated MemTable bytes currently buffered.", float64(snapshot.MemTableBytes))
	registry.SetGauge("mts_memtable_series", "MemTable series currently buffered.", float64(snapshot.MemTableSeries))
	registry.SetGauge("mts_memtable_fields", "MemTable fields currently buffered.", float64(snapshot.MemTableFields))
	registry.SetGauge("mts_memtable_columns", "MemTable typed columns currently buffered.", float64(snapshot.MemTableColumns))
	registry.AddCounter("mts_memtable_flush_triggered_total", "MemTable flushes triggered by memory pressure.", float64(snapshot.MemTableFlushTriggered))
	registry.AddCounter("mts_memtable_out_of_order_samples_total", "MemTable samples appended out of order within a series/field column.", float64(snapshot.MemTableOutOfOrder))
	registry.AddCounter("mts_memtable_duplicate_samples_total", "MemTable samples appended with duplicate timestamps within a series/field column.", float64(snapshot.MemTableDuplicates))
	registry.AddCounter("mts_memtable_appended_samples_total", "MemTable samples appended since open.", float64(snapshot.MemTableAppended))
}

func recordSSTableMetrics(registry *observability.Registry, snapshot productionMetrics) {
	registry.SetGauge("mts_sstable_parts", "SSTable parts currently referenced by manifests.", float64(snapshot.SSTableParts))
	registry.SetGauge("mts_sstable_rows", "SSTable rows currently referenced by manifests.", float64(snapshot.SSTableRows))
	registry.SetGauge("mts_sstable_series", "SSTable series currently referenced by manifests.", float64(snapshot.SSTableSeries))
	registry.SetGauge("mts_sstable_blocks", "SSTable blocks currently referenced by manifests.", float64(snapshot.SSTableBlocks))
	registry.SetGauge("mts_sstable_max_level", "Highest SSTable compaction level currently referenced.", float64(snapshot.SSTableMaxLevel))
	registry.SetGauge("mts_sstable_level0_parts", "Level-0 SSTable parts currently referenced.", float64(snapshot.SSTableLevel0Parts))
	registry.SetGauge("mts_sstable_max_write_seq", "Highest write sequence persisted in SSTables.", float64(snapshot.SSTableMaxWriteSeq))
	registry.SetGauge("mts_sstable_data_bytes", "SSTable data component bytes.", float64(snapshot.SSTableDataBytes))
	registry.SetGauge("mts_sstable_index_bytes", "SSTable index and metadata component bytes.", float64(snapshot.SSTableIndexBytes))
	registry.SetGauge("mts_sstable_total_bytes", "SSTable total component bytes.", float64(snapshot.SSTableTotalBytes))
	registry.SetGauge("mts_sstable_compression_ratio", "Estimated logical data bytes divided by stored data bytes.", snapshot.SSTableCompressionRatio)
}

func recordQueryMetrics(registry *observability.Registry, snapshot model.QueryStats) {
	registry.ObserveHistogram("mts_query_duration_seconds", "Latest query duration.", nanosToSeconds(snapshot.DurationNanos))
	registry.AddCounter("mts_query_samples_returned_total", "Samples returned by the latest query snapshot.", float64(snapshot.SamplesReturned))
	registry.AddCounter("mts_query_samples_read_total", "Samples read by the latest query snapshot.", float64(snapshot.SamplesRead))
	registry.AddCounter("mts_query_parts_scanned_total", "SSTable parts scanned by the latest query snapshot.", float64(snapshot.PartsScanned))
	registry.AddCounter("mts_query_parts_skipped_total", "SSTable parts skipped by the latest query snapshot.", float64(snapshot.PartsSkipped))
	registry.AddCounter("mts_query_errors_total", "Query errors observed in the latest query snapshot.", float64(snapshot.Errors))
	registry.AddCounter("mts_query_budget_errors_total", "Query read budget errors.", float64(snapshot.BudgetErrors))
	registry.AddCounter("mts_query_cancellations_total", "Query cancellations and deadline expirations.", float64(snapshot.Cancellations))
	registry.SetGauge("mts_query_shards_scanned", "Shards scanned by the latest query snapshot.", float64(snapshot.ShardsScanned))
	registry.SetGauge("mts_query_shards_skipped", "Shards skipped by the latest query snapshot.", float64(snapshot.ShardsSkipped))
	registry.SetGauge("mts_query_index_rows_read", "Index rows read by the latest query snapshot.", float64(snapshot.IndexRowsRead))
	registry.SetGauge("mts_query_value_blocks_read", "Value blocks read by the latest query snapshot.", float64(snapshot.ValueBlocksRead))
	registry.SetGauge("mts_query_value_pages_read", "Value pages read by the latest query snapshot.", float64(snapshot.ValuePagesRead))
}

func recordRetentionMetrics(registry *observability.Registry, snapshot productionMetrics) {
	registry.AddCounter("mts_retention_expired_parts_total", "SSTable parts deleted by retention.", float64(snapshot.RetentionExpiredParts))
	registry.AddCounter("mts_retention_deleted_bytes_total", "Bytes deleted by retention.", float64(snapshot.RetentionDeletedBytes))
	registry.AddCounter("mts_retention_delete_errors_total", "Retention delete errors.", float64(snapshot.RetentionDeleteErrors))
}

func recordDownsampleMetrics(registry *observability.Registry, snapshot model.DownsampleStats) {
	registry.SetGauge("mts_downsample_active", "Active downsample policy runs.", float64(snapshot.Active))
	registry.SetGauge("mts_downsample_last_duration_seconds", "Last downsample run duration.", snapshot.LastDuration.Seconds())
	registry.SetGauge("mts_downsample_last_watermark_unix", "Last downsample completed watermark.", float64(snapshot.LastWatermarkUnix))
	registry.AddCounter("mts_downsample_runs_total", "Total downsample policy runs.", float64(snapshot.Total))
	registry.AddCounter("mts_downsample_success_total", "Successful downsample policy runs.", float64(snapshot.Success))
	registry.AddCounter("mts_downsample_failures_total", "Failed downsample policy runs.", float64(snapshot.Failure))
	registry.AddCounter("mts_downsample_windows_processed_total", "Downsample windows processed.", float64(snapshot.WindowsProcessed))
	registry.AddCounter("mts_downsample_points_written_total", "Downsample materialized points written.", float64(snapshot.PointsWritten))
}

func recordDownsamplePolicyMetrics(
	registry *observability.Registry,
	statuses []model.DownsamplePolicyStatus,
) {
	for _, status := range statuses {
		labels := map[string]string{"policy": status.PolicyName}
		active := 0.0
		if status.Active {
			active = 1
		}
		enabled := 0.0
		if status.Enabled {
			enabled = 1
		}
		registry.SetGaugeLabels("mts_downsample_policy_active", labels, "Downsample policy active state.", active)
		registry.SetGaugeLabels("mts_downsample_policy_enabled", labels, "Downsample policy enabled state.", enabled)
		registry.SetGaugeLabels("mts_downsample_policy_lag_seconds", labels, "Downsample policy lag in seconds.", float64(status.LagSeconds))
		registry.SetGaugeLabels("mts_downsample_policy_last_watermark_unix", labels, "Downsample policy completed watermark.", float64(status.CompletedUntilUnix))
		registry.SetGaugeLabels("mts_downsample_policy_next_run_unix", labels, "Downsample policy next run timestamp.", float64(status.NextRunUnix))
		registry.SetGaugeLabels("mts_downsample_policy_last_duration_seconds", labels, "Downsample policy last duration.", status.LastDuration.Seconds())
		registry.AddCounterLabels("mts_downsample_policy_windows_processed_total", labels, "Downsample policy processed windows.", float64(status.WindowsProcessed))
		registry.AddCounterLabels("mts_downsample_policy_points_written_total", labels, "Downsample policy materialized points.", float64(status.PointsWritten))
	}
}

func recordRecoveryMetrics(registry *observability.Registry, snapshot productionMetrics) {
	registry.AddCounter("mts_recovery_issues_total", "Recovery issues found during shard open and maintenance.", float64(snapshot.RecoveryIssues))
	registry.AddCounter("mts_recovery_errors_total", "Recovery issues with concrete errors.", float64(snapshot.RecoveryErrors))
	registry.AddCounter("mts_recovery_fatal_errors_total", "Fatal recovery issues found during shard open.", float64(snapshot.RecoveryFatalErrors))
}

func recordRuntimeMetrics(registry *observability.Registry, snapshot runtimeMetrics) {
	registry.SetGauge("mts_runtime_heap_alloc_bytes", "Go runtime heap allocation bytes.", float64(snapshot.HeapAllocBytes))
	registry.SetGauge("mts_runtime_heap_inuse_bytes", "Go runtime heap in-use bytes.", float64(snapshot.HeapInuseBytes))
	registry.AddCounter("mts_runtime_gc_total", "Go runtime completed GC cycles.", float64(snapshot.GCTotal))
	registry.AddCounter("mts_runtime_gc_pause_total_seconds", "Go runtime cumulative GC pause seconds.", nanosToSeconds(int64(snapshot.GCPauseTotalNanos)))
	registry.SetGauge("mts_runtime_goroutines", "Current goroutine count.", float64(snapshot.Goroutines))
	registry.SetGauge("mts_runtime_fd_open", "Current open file descriptor count.", float64(snapshot.FDOpen))
}

func runtimeMetricsSnapshot() runtimeMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return runtimeMetrics{
		HeapAllocBytes:    mem.HeapAlloc,
		HeapInuseBytes:    mem.HeapInuse,
		GCTotal:           mem.NumGC,
		GCPauseTotalNanos: mem.PauseTotalNs,
		Goroutines:        runtime.NumGoroutine(),
		FDOpen:            countOpenFDs(),
	}
}

func countOpenFDs() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0
	}
	return len(entries)
}

func nanosToSeconds(value int64) float64 {
	return time.Duration(value).Seconds()
}

func recordMaintenanceMetrics(registry *observability.Registry, stats MaintenanceStats) {
	registry.SetGauge("mts_maintenance_compaction_active", "Active compaction tasks.", float64(stats.CompactionActive))
	registry.SetGauge("mts_maintenance_compaction_backlog", "Pending compaction plans.", float64(stats.CompactionBacklog))
	registry.SetGauge("mts_maintenance_downsample_inflight", "In-flight downsample policy runs.", float64(stats.DownsampleInflight))
	registry.SetGauge("mts_maintenance_downsample_active", "Active downsample runs recorded by stats.", float64(stats.DownsampleActive))
	registry.SetGauge("mts_maintenance_downsample_max_concurrent", "Configured max concurrent downsample runs.", float64(stats.DownsampleMaxConcurrent))
	registry.SetGauge("mts_maintenance_compaction_max_concurrent", "Configured max concurrent compaction tasks.", float64(stats.CompactionMaxConcurrent))
	registry.SetGauge("mts_maintenance_error_count", "Shard maintenance errors currently recorded.", float64(stats.MaintenanceErrorCount))
	registry.AddCounter("mts_maintenance_compaction_skipped_total", "Background compaction skips.", float64(stats.CompactionSkipped))
	registry.AddCounter("mts_maintenance_compaction_failures_total", "Compaction failures.", float64(stats.CompactionFailure))
	registry.AddCounter("mts_maintenance_downsample_skipped_total", "Downsample runs skipped by scheduler limits.", float64(stats.DownsampleSkipped))
	registry.AddCounter("mts_maintenance_downsample_failures_total", "Downsample run failures.", float64(stats.DownsampleFailure))
}
