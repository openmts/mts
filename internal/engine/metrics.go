package engine

import "codeberg.org/mts/mts/internal/observability"

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
