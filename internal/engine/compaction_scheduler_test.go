package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestBackgroundCompactionSkipsWhenStorageMemoryBusy(t *testing.T) {
	memoryOptions := model.StorageMemoryOptions{SoftBytesLimit: 10}
	engine := &Engine{
		opts:                model.Options{StorageMemory: memoryOptions},
		memory:              newStorageMemoryLimiter(memoryOptions),
		compactionScheduler: newCompactionScheduler(),
	}
	release, err := engine.memory.Reserve(storageMemoryWrite, 0, 11)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	skipped, reason := engine.shouldSkipBackgroundCompactionLocked()
	if !skipped || reason != compactionSkipMemoryBusy {
		release()
		t.Fatalf("skip=%v reason=%q, want memory busy", skipped, reason)
	}
	engine.recordBackgroundCompactionSkip(reason)
	release()
	stats := engine.CompactionStatsSnapshot()
	if stats.Skipped != 1 || stats.LastSkipReason != compactionSkipMemoryBusy {
		t.Fatalf("stats = %#v, want one memory busy skip", stats)
	}
}

func TestCompactionSchedulerRejectsDuplicateCandidateSignature(t *testing.T) {
	scheduler := newCompactionScheduler()
	if !scheduler.start("level:1|a|b") {
		t.Fatal("first start = false, want true")
	}
	if scheduler.start("level:1|a|b") {
		t.Fatal("duplicate start = true, want false")
	}
	scheduler.finish("level:1|a|b")
	if !scheduler.start("level:1|a|b") {
		t.Fatal("start after finish = false, want true")
	}
	scheduler.finish("level:1|a|b")
	snapshot := scheduler.snapshot()
	if snapshot.DuplicateSkips != 1 {
		t.Fatalf("DuplicateSkips = %d, want 1", snapshot.DuplicateSkips)
	}
}

func TestApplyRetentionWaitsForShardLifecycleLock(t *testing.T) {
	ctx := context.Background()
	files := &fakeFileOps{}
	shard := &Shard{
		opts: ShardOptions{
			Dir: t.TempDir(),
			End: 0,
		},
		deps: shardDeps{files: files},
	}
	engine := &Engine{
		opts:   model.Options{Retention: time.Nanosecond},
		shards: map[string]*Shard{"s": shard},
		logger: nopLogger(),
	}
	shard.lifecycleMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- engine.ApplyRetention(ctx, time.Unix(0, 100))
	}()
	select {
	case err := <-done:
		shard.lifecycleMu.Unlock()
		t.Fatalf("ApplyRetention returned before lifecycle unlock: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	shard.lifecycleMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ApplyRetention() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ApplyRetention did not finish after lifecycle unlock")
	}
	if files.removeAllCalls != 1 {
		t.Fatalf("RemoveAll calls = %d, want 1", files.removeAllCalls)
	}
}
