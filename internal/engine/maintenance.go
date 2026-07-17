package engine

import (
	"context"
)

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

// MaintenanceStats 汇总后台 compact/downsample 的并发、跳过与失败可观测性。
type MaintenanceStats struct {
	CompactionActive        int
	CompactionBacklog       int
	CompactionSkipped       int
	CompactionFailure       int
	CompactionLastSkip      string
	DownsampleActive        int
	DownsampleInflight      int
	DownsampleSkipped       uint64
	DownsampleFailure       int
	DownsampleMaxConcurrent int
	MaintenanceErrorCount   int
}

func (e *Engine) MaintenanceStatsSnapshot() MaintenanceStats {
	compaction := e.CompactionStatsSnapshot()
	downsample := e.DownsampleStatsSnapshot()
	e.downsampleMu.Lock()
	inflight := e.downsampleInflight
	skipped := e.downsampleSkipped
	e.downsampleMu.Unlock()
	maxConcurrent := e.opts.MaxConcurrentDownsample
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentDownsample
	}
	return MaintenanceStats{
		CompactionActive:        compaction.Active,
		CompactionBacklog:       compaction.Backlog,
		CompactionSkipped:       compaction.Skipped,
		CompactionFailure:       compaction.Failure,
		CompactionLastSkip:      compaction.LastSkipReason,
		DownsampleActive:        downsample.Active,
		DownsampleInflight:      inflight,
		DownsampleSkipped:       skipped,
		DownsampleFailure:       downsample.Failure,
		DownsampleMaxConcurrent: maxConcurrent,
		MaintenanceErrorCount:   len(e.MaintenanceErrors(context.Background())),
	}
}
