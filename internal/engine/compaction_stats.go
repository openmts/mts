package engine

import (
	"sync"
	"time"

	"codeberg.org/mts/mts/internal/sstable"
)

type CompactionStats struct {
	Active       int
	Total        int
	Success      int
	Failure      int
	InputParts   int
	OutputParts  int
	InputBytes   int64
	OutputBytes  int64
	LastDuration time.Duration
	LastError    string
}

type compactionStatsRecorder struct {
	mu    sync.RWMutex
	stats CompactionStats
}

func (r *compactionStatsRecorder) begin(parts []sstable.PartMeta) compactionAttempt {
	inputBytes := partMetaBytes(parts)
	r.mu.Lock()
	r.stats.Active++
	r.stats.Total++
	r.stats.InputParts += len(parts)
	r.stats.InputBytes += inputBytes
	r.mu.Unlock()
	return compactionAttempt{recorder: r, started: time.Now()}
}

func (r *compactionStatsRecorder) snapshot() CompactionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

type compactionAttempt struct {
	recorder *compactionStatsRecorder
	started  time.Time
}

func (a compactionAttempt) finish(outputs []sstable.PartMeta, err error) {
	if a.recorder == nil {
		return
	}
	a.recorder.mu.Lock()
	defer a.recorder.mu.Unlock()
	a.recorder.stats.Active--
	a.recorder.stats.LastDuration = time.Since(a.started)
	if err != nil {
		a.recorder.stats.Failure++
		a.recorder.stats.LastError = err.Error()
		return
	}
	a.recorder.stats.Success++
	a.recorder.stats.OutputParts += len(outputs)
	a.recorder.stats.OutputBytes += partMetaBytes(outputs)
	a.recorder.stats.LastError = ""
}

func partMetaBytes(parts []sstable.PartMeta) int64 {
	var total int64
	for _, part := range parts {
		total += int64(part.BlockCount)
	}
	return total
}

func mergeCompactionStats(left CompactionStats, right CompactionStats) CompactionStats {
	left.Active += right.Active
	left.Total += right.Total
	left.Success += right.Success
	left.Failure += right.Failure
	left.InputParts += right.InputParts
	left.OutputParts += right.OutputParts
	left.InputBytes += right.InputBytes
	left.OutputBytes += right.OutputBytes
	if right.LastDuration > left.LastDuration {
		left.LastDuration = right.LastDuration
		left.LastError = right.LastError
	}
	return left
}
