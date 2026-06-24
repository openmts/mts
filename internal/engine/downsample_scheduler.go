package engine

import (
	"context"
	"time"

	"github.com/openmts/mts/internal/model"
)

const downsampleSchedulerTick = 100 * time.Millisecond

func (e *Engine) startDownsampleScheduler() {
	e.downsampleCtx, e.downsampleCancel = context.WithCancel(context.Background())
	e.downsampleStop = make(chan struct{})
	e.downsampleWG.Add(1)
	go e.downsampleSchedulerLoop()
}

func (e *Engine) stopDownsampleScheduler() {
	if e.downsampleCancel != nil {
		e.downsampleCancel()
	}
	e.downsampleStopOnce.Do(func() {
		if e.downsampleStop != nil {
			close(e.downsampleStop)
		}
	})
	e.downsampleWG.Wait()
}

func (e *Engine) downsampleSchedulerLoop() {
	defer e.downsampleWG.Done()
	ticker := time.NewTicker(downsampleSchedulerTick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.scanDownsamplePolicies(e.downsampleContext())
		case <-e.downsampleStop:
			return
		}
	}
}

func (e *Engine) scanDownsamplePolicies(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	policies, err := e.metadata.ListDownsamplePolicies(ctx)
	if err != nil {
		e.logger.Warn("list downsample policies failed", "error", err)
		return
	}
	for _, policy := range policies {
		if !e.shouldRunDownsamplePolicy(ctx, policy) {
			continue
		}
		e.startDownsamplePolicyRun(policy)
	}
}

func (e *Engine) shouldRunDownsamplePolicy(ctx context.Context, policy model.DownsamplePolicy) bool {
	if !policy.Enabled || policy.RefreshInterval <= 0 {
		return false
	}
	watermark, _, err := e.metadata.DownsampleWatermark(ctx, policy.Name)
	if err != nil {
		e.logger.Warn("read downsample watermark failed",
			"policy", policy.Name,
			"error", err,
		)
		return false
	}
	if watermark.LastRunUnix == 0 {
		return true
	}
	return time.Since(time.Unix(0, watermark.LastRunUnix)) >= policy.RefreshInterval
}

func (e *Engine) startDownsamplePolicyRun(policy model.DownsamplePolicy) {
	name := policy.Name
	if !e.acquireDownsamplePolicyRun(name) {
		return
	}
	e.downsampleWG.Add(1)
	go func() {
		defer e.downsampleWG.Done()
		defer e.releaseDownsamplePolicyRun(name)
		ctx, cancel := e.downsampleRunContext(policy)
		defer cancel()
		if _, err := e.RunDownsamplePolicy(ctx, name, time.Duration(time.Now().UnixNano())); err != nil {
			e.logger.Warn("downsample policy run failed",
				"policy", name,
				"error", err,
			)
		}
	}()
}

func (e *Engine) downsampleContext() context.Context {
	if e.downsampleCtx != nil {
		return e.downsampleCtx
	}
	return context.Background()
}

func (e *Engine) downsampleRunContext(policy model.DownsamplePolicy) (context.Context, context.CancelFunc) {
	parent := e.downsampleContext()
	if policy.RunTimeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, policy.RunTimeout)
}

func (e *Engine) acquireDownsamplePolicyRun(name string) bool {
	e.downsampleMu.Lock()
	defer e.downsampleMu.Unlock()
	if _, ok := e.downsampleRunning[name]; ok {
		return false
	}
	e.downsampleRunning[name] = struct{}{}
	return true
}

func (e *Engine) releaseDownsamplePolicyRun(name string) {
	e.downsampleMu.Lock()
	defer e.downsampleMu.Unlock()
	delete(e.downsampleRunning, name)
}
