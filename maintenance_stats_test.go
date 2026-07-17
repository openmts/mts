package mts

import (
	"context"
	"testing"
)

func TestPublicMaintenanceStatsSnapshot(t *testing.T) {
	ctx := context.Background()
	opts := DefaultOptions(t.TempDir())
	opts.MaxConcurrentDownsample = 4
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	stats := eng.MaintenanceStatsSnapshot()
	if stats.DownsampleMaxConcurrent != 4 {
		_ = eng.Close(ctx)
		t.Fatalf("DownsampleMaxConcurrent = %d, want 4", stats.DownsampleMaxConcurrent)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
