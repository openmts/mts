package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEngineDownsampleAPIEnableDisableAndDrop(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        time.Minute,
		Enabled:         false,
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	if err := eng.EnableDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("EnableDownsamplePolicy() error = %v", err)
	}
	policies, err := eng.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || !policies[0].Enabled {
		t.Fatalf("policies after enable = %#v, want enabled", policies)
	}
	if err := eng.DisableDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("DisableDownsamplePolicy() error = %v", err)
	}
	policies, err = eng.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies(disabled) error = %v", err)
	}
	if len(policies) != 1 || policies[0].Enabled {
		t.Fatalf("policies after disable = %#v, want disabled", policies)
	}
	if err := eng.DropDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("DropDownsamplePolicy() error = %v", err)
	}
	policies, err = eng.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies(dropped) error = %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("policies after drop = %#v, want empty", policies)
	}
}

func TestEngineDownsampleAPIRejectsUnknownPolicy(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.EnableDownsamplePolicy(ctx, "missing"); err == nil {
		t.Fatal("EnableDownsamplePolicy(missing) error = nil, want error")
	}
	if err := eng.DisableDownsamplePolicy(ctx, "missing"); err == nil {
		t.Fatal("DisableDownsamplePolicy(missing) error = nil, want error")
	}
}

func TestEngineDownsampleRangeDryRunRepairAndBackfill(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.CreateDownsamplePolicy(ctx, testDownsamplePolicy()); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	dryRun, err := eng.DryRunDownsamplePolicy(ctx, "cpu_1m", 0, int64(2*time.Minute))
	if err != nil {
		t.Fatalf("DryRunDownsamplePolicy() error = %v", err)
	}
	if dryRun.Windows != 2 || dryRun.PointsEstimate != 2 {
		t.Fatalf("dryRun = %#v, want two windows", dryRun)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil {
		t.Fatalf("DownsampleWatermark() error = %v", err)
	}
	if ok && watermark.CompletedUntilUnix != 0 {
		t.Fatalf("watermark after dry-run = %#v, want not advanced", watermark)
	}

	repair, err := eng.RepairDownsamplePolicy(ctx, "cpu_1m", 0, int64(time.Minute))
	if err != nil {
		t.Fatalf("RepairDownsamplePolicy() error = %v", err)
	}
	if repair.PointsWritten != 1 || repair.CompletedUntilUnix != 0 {
		t.Fatalf("repair = %#v, want write without watermark advance", repair)
	}

	backfill, err := eng.RunDownsamplePolicyRange(
		ctx,
		"cpu_1m",
		0,
		int64(2*time.Minute),
		model.DownsampleRangeOptions{AdvanceWatermark: true},
	)
	if err != nil {
		t.Fatalf("RunDownsamplePolicyRange() error = %v", err)
	}
	if backfill.CompletedUntilUnix != int64(2*time.Minute) {
		t.Fatalf("backfill = %#v, want watermark at 2m", backfill)
	}
}

func TestEngineGetDownsamplePolicy(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := model.DownsamplePolicy{
		Name:              "cpu_get",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        time.Minute,
		Enabled:         true,
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	got, err := eng.GetDownsamplePolicy(ctx, "cpu_get")
	if err != nil {
		t.Fatalf("GetDownsamplePolicy() error = %v", err)
	}
	if got.Name != "cpu_get" || got.TargetMeasurement != "cpu" {
		t.Fatalf("GetDownsamplePolicy() = %#v", got)
	}
	if _, err := eng.GetDownsamplePolicy(ctx, "missing"); err == nil {
		t.Fatal("GetDownsamplePolicy(missing) error = nil, want error")
	}
}
