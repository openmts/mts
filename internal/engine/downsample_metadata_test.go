package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestLocalMetadataStoreDownsamplePersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := OpenLocalMetadataStore(dir)
	if err != nil {
		t.Fatalf("OpenLocalMetadataStore() error = %v", err)
	}
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
			As:       "avg_usage",
		}},
		GroupByTags:     []string{"host"},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        3 * time.Minute,
		Enabled:         true,
	}
	if err := store.UpsertDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("UpsertDownsamplePolicy() error = %v", err)
	}
	watermark := model.DownsampleWatermark{
		PolicyName:         "cpu_1m",
		CompletedUntilUnix: int64(2 * time.Minute),
		LastRunUnix:        int64(3 * time.Minute),
		LastSuccessUnix:    int64(4 * time.Minute),
	}
	if err := store.UpdateDownsampleWatermark(ctx, watermark); err != nil {
		t.Fatalf("UpdateDownsampleWatermark() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := OpenLocalMetadataStore(dir)
	if err != nil {
		t.Fatalf("OpenLocalMetadataStore(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	policies, err := reopened.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "cpu_1m" ||
		policies[0].Functions[0].As != "avg_usage" {
		t.Fatalf("policies = %#v, want persisted cpu_1m", policies)
	}
	got, ok, err := reopened.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil {
		t.Fatalf("DownsampleWatermark() error = %v", err)
	}
	if !ok || got.CompletedUntilUnix != watermark.CompletedUntilUnix {
		t.Fatalf("watermark = %#v ok=%v, want %#v", got, ok, watermark)
	}
}

func TestLocalMetadataStoreDownsampleCanceledBranches(t *testing.T) {
	store, err := OpenLocalMetadataStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenLocalMetadataStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	policy := model.DownsamplePolicy{Name: "cpu_1m"}
	if err := store.UpsertDownsamplePolicy(ctx, policy); !errors.Is(err, context.Canceled) {
		t.Fatalf("UpsertDownsamplePolicy(cancelled) error = %v", err)
	}
	if err := store.DropDownsamplePolicy(ctx, "cpu_1m"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DropDownsamplePolicy(cancelled) error = %v", err)
	}
	if _, err := store.ListDownsamplePolicies(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListDownsamplePolicies(cancelled) error = %v", err)
	}
	if _, _, err := store.DownsampleWatermark(ctx, "cpu_1m"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DownsampleWatermark(cancelled) error = %v", err)
	}
	watermark := model.DownsampleWatermark{PolicyName: "cpu_1m"}
	if err := store.UpdateDownsampleWatermark(ctx, watermark); !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateDownsampleWatermark(cancelled) error = %v", err)
	}
}

func TestCreateDownsamplePolicyRejectsIncompatibleUpdateUntilReset(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := testDownsamplePolicy()
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	changed := policy
	changed.Functions = []model.DownsampleFunction{{
		Function: "max",
		Field:    "usage",
	}}
	if err := eng.CreateDownsamplePolicy(ctx, changed); err == nil {
		t.Fatal("CreateDownsamplePolicy(incompatible) error = nil, want error")
	}
	if err := eng.ResetDownsamplePolicy(ctx, "cpu_1m", model.DownsampleReset{
		AllowPolicyReplace: true,
	}); err != nil {
		t.Fatalf("ResetDownsamplePolicy() error = %v", err)
	}
	if err := eng.CreateDownsamplePolicy(ctx, changed); err != nil {
		t.Fatalf("CreateDownsamplePolicy(after reset) error = %v", err)
	}
}

func TestDryRunDownsamplePolicyEstimatesGroupsSamplesAndPoints(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	points := []model.Point{
		downsampleEstimatePoint("a", 0, 1),
		downsampleEstimatePoint("a", int64(time.Second), 2),
		downsampleEstimatePoint("b", 0, 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	policy := testDownsamplePolicy()
	policy.InitialStartTime = 0
	policy.Lookback = 2 * time.Minute
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	result, err := eng.DryRunDownsamplePolicy(ctx, "cpu_1m", 0, int64(2*time.Minute))
	if err != nil {
		t.Fatalf("DryRunDownsamplePolicy() error = %v", err)
	}
	if result.Windows != 2 || result.GroupsEstimate != 2 ||
		result.SamplesEstimate != 3 || result.PointsEstimate != 4 ||
		!result.EstimateComplete {
		t.Fatalf("dry-run result = %#v, want windows/groups/samples/points estimates", result)
	}
}

func TestDropDownsamplePolicyWithOptionsCleansTargetData(t *testing.T) {
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
	if _, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 2*time.Minute); err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if err := eng.DropDownsamplePolicyWithOptions(ctx, "cpu_1m", model.DownsampleDropOptions{
		CleanupTarget:    true,
		CleanupStartUnix: 0,
		CleanupEndUnix:   int64(2 * time.Minute),
	}); err != nil {
		t.Fatalf("DropDownsamplePolicyWithOptions() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         int64(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("target rows after cleanup = %#v, want none", rows)
	}
}

func TestResetDownsamplePolicyCleansTargetDataAndRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.DropDownsamplePolicyWithOptions(ctx, "", model.DownsampleDropOptions{}); err == nil {
		t.Fatal("DropDownsamplePolicyWithOptions(empty) error = nil, want error")
	}
	if err := eng.ResetDownsamplePolicy(ctx, "", model.DownsampleReset{}); err == nil {
		t.Fatal("ResetDownsamplePolicy(empty) error = nil, want error")
	}
	if err := eng.ResetDownsamplePolicy(ctx, "cpu_1m", model.DownsampleReset{
		CompletedUntilUnix: -1,
	}); err == nil {
		t.Fatal("ResetDownsamplePolicy(negative) error = nil, want error")
	}
	if err := eng.CreateDownsamplePolicy(ctx, testDownsamplePolicy()); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	if _, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 2*time.Minute); err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if err := eng.ResetDownsamplePolicy(ctx, "cpu_1m", model.DownsampleReset{
		CompletedUntilUnix: int64(time.Minute),
		AllowPolicyReplace: true,
		CleanupTarget:      true,
		CleanupStartUnix:   0,
		CleanupEndUnix:     int64(2 * time.Minute),
	}); err != nil {
		t.Fatalf("ResetDownsamplePolicy(cleanup) error = %v", err)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil || !ok {
		t.Fatalf("DownsampleWatermark() = %#v ok=%v err=%v", watermark, ok, err)
	}
	if watermark.CompletedUntilUnix != int64(time.Minute) || !watermark.AllowPolicyReplace {
		t.Fatalf("watermark = %#v, want reset completion and replace allowance", watermark)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         int64(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("target rows after reset cleanup = %#v, want none", rows)
	}
}

func downsampleEstimatePoint(host string, timestamp int64, usage float64) model.Point {
	return model.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": host},
		Timestamp:       timestamp,
		Fields:          map[string]model.FieldValue{"usage": model.Float64Value(usage)},
	}
}
