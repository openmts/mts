package engine

import (
	"sync"
	"time"

	"github.com/openmts/mts/internal/model"
)

type downsampleStatsRecorder struct {
	mu       sync.RWMutex
	stats    model.DownsampleStats
	policies map[string]model.DownsamplePolicyRuntimeStats
}

type downsampleStatsAttempt struct {
	recorder *downsampleStatsRecorder
	policy   string
	started  time.Time
}

func (r *downsampleStatsRecorder) begin(policy string) downsampleStatsAttempt {
	started := time.Now()
	r.mu.Lock()
	if r.policies == nil {
		r.policies = make(map[string]model.DownsamplePolicyRuntimeStats)
	}
	r.stats.Active++
	r.stats.Total++
	r.stats.LastPolicy = policy
	policyStats := r.policies[policy]
	policyStats.Active++
	policyStats.Total++
	policyStats.LastRunUnix = started.UnixNano()
	r.policies[policy] = policyStats
	r.mu.Unlock()
	return downsampleStatsAttempt{
		recorder: r,
		policy:   policy,
		started:  started,
	}
}

func (r *downsampleStatsRecorder) snapshot() model.DownsampleStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

func (r *downsampleStatsRecorder) policySnapshot(
	policy string,
) model.DownsamplePolicyRuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.policies == nil {
		return model.DownsamplePolicyRuntimeStats{}
	}
	return r.policies[policy]
}

func (a downsampleStatsAttempt) finishSuccess(result DownsampleRunResult) {
	if a.recorder == nil {
		return
	}
	a.recorder.mu.Lock()
	defer a.recorder.mu.Unlock()
	a.recorder.stats.Active--
	a.recorder.stats.Success++
	a.recorder.stats.WindowsProcessed += result.WindowsProcessed
	a.recorder.stats.PointsWritten += result.PointsWritten
	a.recorder.stats.LastDuration = time.Since(a.started)
	a.recorder.stats.LastWatermarkUnix = result.CompletedUntilUnix
	a.recorder.stats.LastPolicy = a.policy
	a.recorder.stats.LastError = ""
	a.recorder.finishPolicyLocked(a.policy, result, true, nil, a.started)
}

func (a downsampleStatsAttempt) finishFailure(result DownsampleRunResult, err error) {
	if a.recorder == nil {
		return
	}
	a.recorder.mu.Lock()
	defer a.recorder.mu.Unlock()
	a.recorder.stats.Active--
	a.recorder.stats.Failure++
	a.recorder.stats.WindowsProcessed += result.WindowsProcessed
	a.recorder.stats.PointsWritten += result.PointsWritten
	a.recorder.stats.LastDuration = time.Since(a.started)
	a.recorder.stats.LastWatermarkUnix = result.CompletedUntilUnix
	a.recorder.stats.LastPolicy = a.policy
	if err != nil {
		a.recorder.stats.LastError = err.Error()
	}
	a.recorder.finishPolicyLocked(a.policy, result, false, err, a.started)
}

func (r *downsampleStatsRecorder) finishPolicyLocked(
	policy string,
	result DownsampleRunResult,
	success bool,
	err error,
	started time.Time,
) {
	if r.policies == nil {
		r.policies = make(map[string]model.DownsamplePolicyRuntimeStats)
	}
	stats := r.policies[policy]
	if stats.Active > 0 {
		stats.Active--
	}
	if success {
		stats.Success++
		stats.LastError = ""
		stats.LastSuccessUnix = result.CompletedUnix
	} else {
		stats.Failure++
		if err != nil {
			stats.LastError = err.Error()
		}
	}
	stats.WindowsProcessed += result.WindowsProcessed
	stats.PointsWritten += result.PointsWritten
	stats.LastDuration = time.Since(started)
	stats.LastWatermarkUnix = result.CompletedUntilUnix
	stats.LastRunUnix = result.CompletedUnix
	r.policies[policy] = stats
}
