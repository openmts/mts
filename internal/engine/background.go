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
			_ = e.Compact(context.Background())
		case <-e.compactStop:
			return
		}
	}
}
