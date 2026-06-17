package engine

import (
	"strconv"
	"sync"
	"time"

	"codeberg.org/mts/mts/internal/sstable"
)

type CompactionStats struct {
	Active          int
	Backlog         int
	Skipped         int
	Total           int
	Success         int
	Failure         int
	InputParts      int
	OutputParts     int
	InputBytes      int64
	OutputBytes     int64
	DroppedRows     int
	OverlapCount    int
	MaxScore        float64
	LastReason      string
	LastLevel       int
	LastOutputLevel int
	LastDuration    time.Duration
	LastError       string
	LastSkipReason  string
	LastTask        CompactionTaskStatus
	SafeDeleteParts int
}

type compactionStatsRecorder struct {
	mu       sync.RWMutex
	stats    CompactionStats
	nextTask uint64
}

type CompactionTaskStatus struct {
	ID          string
	State       string
	Level       int
	OutputLevel int
	Reason      string
	Score       float64
	StartedAt   time.Time
	FinishedAt  time.Time
	Duration    time.Duration
	InputParts  int
	OutputParts int
	InputBytes  int64
	OutputBytes int64
	DroppedRows int
	Error       string
}

const (
	compactionTaskNoop      = "noop"
	compactionTaskRunning   = "running"
	compactionTaskSucceeded = "succeeded"
	compactionTaskFailed    = "failed"
)

type CompactionResult struct {
	State       string
	Duration    time.Duration
	Shards      int
	InputParts  int
	OutputParts int
	InputBytes  int64
	OutputBytes int64
	DroppedRows int
	Error       string
	LastTask    CompactionTaskStatus
}

func (r *compactionStatsRecorder) begin(parts []sstable.PartMeta) compactionAttempt {
	return r.beginPlan(compactionPlan{
		candidates: parts,
		inputBytes: partMetaBytes(parts),
	})
}

func (r *compactionStatsRecorder) beginPlan(plan compactionPlan) compactionAttempt {
	inputBytes := plan.inputBytes
	if inputBytes == 0 {
		inputBytes = partMetaBytes(plan.candidates)
	}
	started := time.Now()
	r.mu.Lock()
	r.nextTask++
	task := CompactionTaskStatus{
		ID:          compactionTaskID(r.nextTask),
		State:       compactionTaskRunning,
		Level:       plan.level,
		OutputLevel: plan.outputLevel,
		Reason:      plan.reason,
		Score:       plan.score,
		StartedAt:   started,
		InputParts:  len(plan.candidates),
		InputBytes:  inputBytes,
	}
	r.stats.Active++
	r.stats.Total++
	r.stats.InputParts += len(plan.candidates)
	r.stats.InputBytes += inputBytes
	r.stats.LastReason = plan.reason
	r.stats.LastLevel = plan.level
	r.stats.LastOutputLevel = plan.outputLevel
	if plan.score > r.stats.MaxScore {
		r.stats.MaxScore = plan.score
	}
	r.stats.LastTask = task
	r.mu.Unlock()
	return compactionAttempt{recorder: r, started: started, task: task}
}

func (r *compactionStatsRecorder) snapshot() CompactionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

func (r *compactionStatsRecorder) recordSkip(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.Skipped++
	r.stats.LastSkipReason = reason
}

func (r *compactionStatsRecorder) recordSafeDeleteParts(count int) {
	if count <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.SafeDeleteParts += count
}

type compactionAttempt struct {
	recorder *compactionStatsRecorder
	started  time.Time
	task     CompactionTaskStatus
}

func (a compactionAttempt) finish(outputs []sstable.PartMeta, err error) {
	a.finishWithRows(outputs, 0, err)
}

func (a compactionAttempt) finishWithRows(outputs []sstable.PartMeta, droppedRows int, err error) {
	if a.recorder == nil {
		return
	}
	outputBytes := partMetaBytes(outputs)
	finished := time.Now()
	duration := finished.Sub(a.started)
	task := a.task
	task.FinishedAt = finished
	task.Duration = duration
	task.OutputParts = len(outputs)
	task.OutputBytes = outputBytes
	task.DroppedRows = droppedRows
	a.recorder.mu.Lock()
	defer a.recorder.mu.Unlock()
	a.recorder.stats.Active--
	a.recorder.stats.LastDuration = duration
	a.recorder.stats.DroppedRows += droppedRows
	if err != nil {
		task.State = compactionTaskFailed
		task.Error = err.Error()
		a.recorder.stats.Failure++
		a.recorder.stats.LastError = err.Error()
		a.recorder.stats.LastTask = task
		return
	}
	task.State = compactionTaskSucceeded
	a.recorder.stats.Success++
	a.recorder.stats.OutputParts += len(outputs)
	a.recorder.stats.OutputBytes += outputBytes
	a.recorder.stats.LastError = ""
	a.recorder.stats.LastTask = task
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
	left.Backlog += right.Backlog
	left.Skipped += right.Skipped
	left.Total += right.Total
	left.Success += right.Success
	left.Failure += right.Failure
	left.InputParts += right.InputParts
	left.OutputParts += right.OutputParts
	left.InputBytes += right.InputBytes
	left.OutputBytes += right.OutputBytes
	left.DroppedRows += right.DroppedRows
	left.OverlapCount += right.OverlapCount
	left.SafeDeleteParts += right.SafeDeleteParts
	if right.MaxScore > left.MaxScore {
		left.MaxScore = right.MaxScore
	}
	if right.LastDuration > left.LastDuration {
		left.LastDuration = right.LastDuration
		left.LastError = right.LastError
		left.LastReason = right.LastReason
		left.LastLevel = right.LastLevel
		left.LastOutputLevel = right.LastOutputLevel
		left.LastTask = right.LastTask
	}
	if right.LastSkipReason != "" {
		left.LastSkipReason = right.LastSkipReason
	}
	return left
}

func compactionTaskID(value uint64) string {
	return "compaction-" + strconv.FormatUint(value, 10)
}

func mergeCompactionSchedulerStats(
	stats CompactionStats,
	scheduler compactionSchedulerSnapshot,
) CompactionStats {
	stats.Skipped += scheduler.TotalSkips
	if scheduler.LastSkipReason != "" {
		stats.LastSkipReason = scheduler.LastSkipReason
	}
	return stats
}
