package engine

import (
	"context"
	"time"
)

func (e *Engine) startBackgroundCompaction() {
	if !e.opts.Compaction.Enabled || e.opts.Compaction.BackgroundInterval <= 0 {
		return
	}
	e.compactStop = make(chan struct{})
	e.compactWG.Add(1)
	go e.backgroundCompactionLoop(e.opts.Compaction.BackgroundInterval)
}

func (e *Engine) stopBackgroundCompaction() {
	if e.compactStop == nil {
		return
	}
	e.compactStopOnce.Do(func() {
		close(e.compactStop)
	})
	e.compactWG.Wait()
}

func (e *Engine) backgroundCompactionLoop(interval time.Duration) {
	defer e.compactWG.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := e.compactBackground(context.Background()); err != nil {
				e.logger.Warn("background compaction failed", "error", err)
			}
		case <-e.compactStop:
			return
		}
	}
}

func (e *Engine) compactBackground(ctx context.Context) error {
	e.mu.Lock()
	if skipped, reason := e.shouldSkipBackgroundCompactionLocked(); skipped {
		e.mu.Unlock()
		e.recordBackgroundCompactionSkip(reason)
		return nil
	}
	e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.Compact(ctx)
}

func (e *Engine) shouldSkipBackgroundCompactionLocked() (bool, string) {
	if e == nil || e.memory == nil {
		return false, ""
	}
	snapshot := e.memory.Snapshot(e.storageMemoryActiveLocked())
	if snapshot.SoftBytesLimit > 0 && snapshot.CurrentBytes >= snapshot.SoftBytesLimit {
		return true, compactionSkipMemoryBusy
	}
	return false, ""
}

func (e *Engine) recordBackgroundCompactionSkip(reason string) {
	if e == nil || e.compactionScheduler == nil {
		return
	}
	e.compactionScheduler.recordSkip(reason)
}
