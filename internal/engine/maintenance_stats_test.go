package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestDownsampleGlobalConcurrencyLimitAndSkipCount(t *testing.T) {
	eng := &Engine{
		opts:              model.Options{MaxConcurrentDownsample: 1},
		downsampleRunning: make(map[string]struct{}),
	}
	if !eng.acquireDownsamplePolicyRun("p1") {
		t.Fatal("first acquire = false, want true")
	}
	if eng.acquireDownsamplePolicyRun("p2") {
		t.Fatal("second acquire under limit=1 = true, want false")
	}
	if eng.downsampleSkipped != 1 {
		t.Fatalf("downsampleSkipped = %d, want 1", eng.downsampleSkipped)
	}
	if eng.downsampleInflight != 1 {
		t.Fatalf("downsampleInflight = %d, want 1", eng.downsampleInflight)
	}
	eng.releaseDownsamplePolicyRun("p1")
	if eng.downsampleInflight != 0 {
		t.Fatalf("downsampleInflight after release = %d, want 0", eng.downsampleInflight)
	}
	if !eng.acquireDownsamplePolicyRun("p2") {
		t.Fatal("acquire after release = false, want true")
	}
	eng.releaseDownsamplePolicyRun("p2")
}

func TestDownsampleDuplicatePolicyStillSkipped(t *testing.T) {
	eng := &Engine{
		opts:              model.Options{MaxConcurrentDownsample: 4},
		downsampleRunning: make(map[string]struct{}),
	}
	if !eng.acquireDownsamplePolicyRun("same") {
		t.Fatal("first acquire = false, want true")
	}
	if eng.acquireDownsamplePolicyRun("same") {
		t.Fatal("duplicate acquire = true, want false")
	}
	if eng.downsampleSkipped != 1 {
		t.Fatalf("downsampleSkipped = %d, want 1", eng.downsampleSkipped)
	}
	eng.releaseDownsamplePolicyRun("same")
}

func TestBackgroundCompactionUsesCancelableContext(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Compaction: model.CompactionOptions{
			Enabled:            true,
			BackgroundInterval: time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if eng.compactCtx == nil {
		_ = eng.Close(ctx)
		t.Fatal("compactCtx = nil, want cancelable context")
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := eng.compactCtx.Err(); err == nil {
		t.Fatal("compactCtx.Err() = nil after Close, want canceled")
	}
}

func TestMaintenanceStatsSnapshotIncludesLimitsAndSkips(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:                    t.TempDir(),
		ShardDuration:           time.Hour,
		MaxConcurrentDownsample: 3,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// force skip counters
	eng.downsampleMu.Lock()
	eng.downsampleSkipped = 5
	eng.downsampleInflight = 1
	eng.downsampleMu.Unlock()
	eng.compactionScheduler.recordSkip(compactionSkipMemoryBusy)

	stats := eng.MaintenanceStatsSnapshot()
	if stats.DownsampleMaxConcurrent != 3 {
		_ = eng.Close(ctx)
		t.Fatalf("DownsampleMaxConcurrent = %d, want 3", stats.DownsampleMaxConcurrent)
	}
	if stats.DownsampleSkipped != 5 {
		_ = eng.Close(ctx)
		t.Fatalf("DownsampleSkipped = %d, want 5", stats.DownsampleSkipped)
	}
	if stats.DownsampleInflight != 1 {
		_ = eng.Close(ctx)
		t.Fatalf("DownsampleInflight = %d, want 1", stats.DownsampleInflight)
	}
	if stats.CompactionSkipped < 1 {
		_ = eng.Close(ctx)
		t.Fatalf("CompactionSkipped = %d, want >=1", stats.CompactionSkipped)
	}
	metrics := eng.MetricsSnapshot()
	found := false
	for _, metric := range metrics {
		if metric.Name == "mts_maintenance_downsample_max_concurrent" {
			found = true
			if metric.Value != 3 {
				_ = eng.Close(ctx)
				t.Fatalf("metric value = %v, want 3", metric.Value)
			}
		}
	}
	if !found {
		_ = eng.Close(ctx)
		t.Fatal("missing mts_maintenance_downsample_max_concurrent metric")
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
