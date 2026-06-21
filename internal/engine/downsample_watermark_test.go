package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestRunDownsamplePolicyAdvancesWatermarkAndRefreshesLookback(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := testDownsamplePolicy()
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 5*time.Minute)
	if err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if result.WindowsProcessed != 5 || result.PointsWritten != 5 {
		t.Fatalf("result = %#v, want five windows and points", result)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(ctx, "cpu_1m")
	if err != nil || !ok {
		t.Fatalf("DownsampleWatermark() = %#v ok=%v err=%v", watermark, ok, err)
	}
	if watermark.CompletedUntilUnix != int64(5*time.Minute) {
		t.Fatalf("CompletedUntilUnix = %d, want 5m", watermark.CompletedUntilUnix)
	}

	late := model.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       int64(2 * time.Minute),
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(100),
		},
	}
	if err := eng.Write(ctx, []model.Point{late}, model.WriteOptions{}); err != nil {
		t.Fatalf("Write(late) error = %v", err)
	}
	result, err = eng.RunDownsamplePolicy(ctx, "cpu_1m", 6*time.Minute)
	if err != nil {
		t.Fatalf("RunDownsamplePolicy(refresh) error = %v", err)
	}
	if result.CompletedUntilUnix != int64(6*time.Minute) {
		t.Fatalf("refresh CompletedUntilUnix = %d, want 6m", result.CompletedUntilUnix)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          []string{"avg_usage"},
		StartTime:       int64(2 * time.Minute),
		EndTime:         int64(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(refreshed) error = %v", err)
	}
	refreshed := false
	for _, row := range rows {
		if row.Timestamp != int64(2*time.Minute) {
			continue
		}
		refreshed = row.Fields["avg_usage"].Float64 == 100
	}
	if !refreshed {
		t.Fatalf("rows = %#v, want refreshed 2m avg_usage=100", rows)
	}
}

func TestRunDownsamplePolicySkipsIncompleteWindow(t *testing.T) {
	ctx := context.Background()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := testDownsamplePolicy()
	policy.Delay = time.Minute
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 90*time.Second)
	if err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if result.WindowsProcessed != 0 || result.CompletedUntilUnix != 0 {
		t.Fatalf("result = %#v, want no complete delayed windows", result)
	}
}

func TestDownsampleWindowsToRunUsesInitialStartAndMarksRefresh(t *testing.T) {
	policy := testDownsamplePolicy()
	policy.InitialStartTime = int64(2 * time.Minute)
	windows := downsampleWindowsToRun(policy, model.DownsampleWatermark{}, 5*time.Minute)
	if len(windows) != 3 {
		t.Fatalf("windows = %#v, want three initial backfill windows", windows)
	}
	if windows[0].start != int64(2*time.Minute) || windows[0].refresh {
		t.Fatalf("first window = %#v, want 2m advance", windows[0])
	}
	if windows[2].start != int64(4*time.Minute) || windows[2].refresh {
		t.Fatalf("last window = %#v, want 4m advance", windows[2])
	}

	policy.InitialStartTime = 0
	policy.Lookback = 2 * time.Minute
	watermark := model.DownsampleWatermark{CompletedUntilUnix: int64(5 * time.Minute)}
	windows = downsampleWindowsToRun(policy, watermark, 7*time.Minute)
	if len(windows) != 4 {
		t.Fatalf("refresh windows = %#v, want four windows", windows)
	}
	if !windows[0].refresh || !windows[1].refresh {
		t.Fatalf("windows = %#v, want first two refresh windows", windows)
	}
	if windows[2].refresh || windows[3].refresh {
		t.Fatalf("windows = %#v, want last two advance windows", windows)
	}
}

func TestRunDownsamplePolicyKeepsCheckpointAfterLaterWindowFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := openEngineWithRawDownsampleSamples(t)
	defer func() {
		decorateColumnDataHook = nil
		if err := eng.Close(context.Background()); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	policy := testDownsamplePolicy()
	policy.CheckpointInterval = 1
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	decorations := 0
	decorateColumnDataHook = func() {
		decorations++
		if decorations == 2 {
			cancel()
		}
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", 3*time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunDownsamplePolicy() error = %v, want context.Canceled", err)
	}
	if result.CompletedUntilUnix != int64(time.Minute) {
		t.Fatalf("result.CompletedUntilUnix = %d, want first checkpoint", result.CompletedUntilUnix)
	}
	watermark, ok, err := eng.metadata.DownsampleWatermark(context.Background(), "cpu_1m")
	if err != nil || !ok {
		t.Fatalf("DownsampleWatermark() = %#v ok=%v err=%v", watermark, ok, err)
	}
	if watermark.CompletedUntilUnix != int64(time.Minute) || watermark.LastError == "" {
		t.Fatalf("watermark = %#v, want checkpoint and error", watermark)
	}
}

func openEngineWithRawDownsampleSamples(t *testing.T) *Engine {
	t.Helper()
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 5)
	for minute := int64(0); minute < 5; minute++ {
		points = append(points, model.Point{
			Database:        "metrics",
			RetentionPolicy: "autogen",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "a"},
			Timestamp:       minute * int64(time.Minute),
			Fields: map[string]model.FieldValue{
				"usage": model.Float64Value(float64(minute)),
			},
		})
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write(raw) error = %v close = %v", err, closeErr)
	}
	return eng
}

func testDownsamplePolicy() model.DownsamplePolicy {
	return model.DownsamplePolicy{
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
		GroupByTags:     []string{"host"},
		Delay:           0,
		RefreshInterval: time.Minute,
		Lookback:        5 * time.Minute,
		Enabled:         true,
	}
}
