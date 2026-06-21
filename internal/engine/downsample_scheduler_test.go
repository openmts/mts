package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestDownsampleSchedulerRunsEnabledPolicyAndStopsOnClose(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	interval := 100 * time.Millisecond
	windowStart := alignDownsampleWindow(time.Now().Add(-500*time.Millisecond).UnixNano(), interval)
	point := model.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       windowStart + int64(10*time.Millisecond),
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(42),
		},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write(raw) error = %v close = %v", err, closeErr)
	}
	policy := model.DownsamplePolicy{
		Name:              "cpu_100ms",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_100ms",
		TargetMeasurement: "cpu",
		Interval:          interval,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		GroupByTags:     []string{"host"},
		Delay:           0,
		RefreshInterval: 10 * time.Millisecond,
		Lookback:        2 * time.Second,
		Enabled:         true,
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CreateDownsamplePolicy() error = %v close = %v", err, closeErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := eng.QueryRows(ctx, model.Query{
			Database:        "metrics",
			RetentionPolicy: "rp_100ms",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "a"},
			Fields:          []string{"avg_usage"},
			StartTime:       windowStart,
			EndTime:         windowStart + int64(interval),
		})
		if err == nil && hasDownsampleValue(rows, windowStart, 42) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_100ms",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          []string{"avg_usage"},
		StartTime:       windowStart,
		EndTime:         windowStart + int64(interval),
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows(target) error = %v close = %v", err, closeErr)
	}
	if !hasDownsampleValue(rows, windowStart, 42) {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want scheduler materialized value close = %v", rows, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDownsampleSchedulerPreventsDuplicatePolicyRuns(t *testing.T) {
	eng := &Engine{downsampleRunning: make(map[string]struct{})}
	if !eng.acquireDownsamplePolicyRun("cpu_1m") {
		t.Fatal("first acquire = false, want true")
	}
	if eng.acquireDownsamplePolicyRun("cpu_1m") {
		t.Fatal("duplicate acquire = true, want false")
	}
	eng.releaseDownsamplePolicyRun("cpu_1m")
	if !eng.acquireDownsamplePolicyRun("cpu_1m") {
		t.Fatal("acquire after release = false, want true")
	}
}

func TestDownsampleSchedulerCloseCancelsRootContext(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if eng.downsampleCtx == nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("downsampleCtx = nil close = %v", closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := eng.downsampleCtx.Err(); err != context.Canceled {
		t.Fatalf("downsampleCtx.Err() = %v, want context.Canceled", err)
	}
}

func TestDownsampleRunContextUsesPolicyTimeout(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := &Engine{downsampleCtx: parent}
	runCtx, runCancel := eng.downsampleRunContext(model.DownsamplePolicy{
		RunTimeout: time.Nanosecond,
	})
	defer runCancel()
	select {
	case <-runCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("run context did not expire")
	}
	if err := runCtx.Err(); err != context.DeadlineExceeded {
		t.Fatalf("runCtx.Err() = %v, want context.DeadlineExceeded", err)
	}
}

func hasDownsampleValue(rows []model.Row, timestamp int64, value float64) bool {
	for _, row := range rows {
		if row.Timestamp != timestamp {
			continue
		}
		if row.Fields["avg_usage"].Float64 == value {
			return true
		}
	}
	return false
}
