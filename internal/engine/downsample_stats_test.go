package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/observability"
)

func TestDownsampleStatsExposeWatermarkAndFailures(t *testing.T) {
	var stats downsampleStatsRecorder
	success := stats.begin("cpu_1m")
	success.finishSuccess(DownsampleRunResult{
		PolicyName:         "cpu_1m",
		WindowsProcessed:   2,
		PointsWritten:      10,
		CompletedUntilUnix: int64(2 * time.Minute),
	})
	failure := stats.begin("cpu_1m")
	failure.finishFailure(DownsampleRunResult{
		PolicyName:         "cpu_1m",
		CompletedUntilUnix: int64(2 * time.Minute),
	}, errors.New("query failed"))

	snapshot := stats.snapshot()
	if snapshot.Total != 2 || snapshot.Success != 1 || snapshot.Failure != 1 {
		t.Fatalf("snapshot counts = %#v, want total=2 success=1 failure=1", snapshot)
	}
	if snapshot.WindowsProcessed != 2 || snapshot.PointsWritten != 10 {
		t.Fatalf("snapshot work = %#v, want windows=2 points=10", snapshot)
	}
	if snapshot.LastWatermarkUnix != int64(2*time.Minute) ||
		snapshot.LastError != "query failed" {
		t.Fatalf("snapshot last = %#v, want watermark and error", snapshot)
	}
}

func TestDownsampleMetricsAndHealthExposeStats(t *testing.T) {
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
	attempt := eng.downsampleStats.begin("cpu_1m")
	attempt.finishFailure(DownsampleRunResult{
		PolicyName:         "cpu_1m",
		CompletedUntilUnix: int64(time.Minute),
	}, errors.New("write failed"))

	names := metricNameSet(eng.MetricsSnapshot())
	for _, name := range []string{
		"mts_downsample_active",
		"mts_downsample_runs_total",
		"mts_downsample_failures_total",
		"mts_downsample_last_watermark_unix",
		"mts_downsample_windows_processed_total",
		"mts_downsample_points_written_total",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("metric %s missing in %#v", name, names)
		}
	}
	health := eng.HealthSnapshot()
	if !healthCheckStatus(health, "downsample", "degraded") {
		t.Fatalf("health = %#v, want downsample degraded", health)
	}
}

func TestDownsamplePolicyStatusesAndMetrics(t *testing.T) {
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
	policy := testDownsamplePolicy()
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	attempt := eng.downsampleStats.begin("cpu_1m")
	attempt.finishSuccess(DownsampleRunResult{
		PolicyName:         "cpu_1m",
		WindowsProcessed:   3,
		PointsWritten:      9,
		StartedUnix:        int64(time.Minute),
		CompletedUnix:      int64(2 * time.Minute),
		CompletedUntilUnix: int64(2 * time.Minute),
	})
	statuses, err := eng.DownsamplePolicyStatuses(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("DownsamplePolicyStatuses() error = %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("statuses = %#v, want one status", statuses)
	}
	status := statuses[0]
	if status.PolicyName != "cpu_1m" ||
		status.CompletedUntilUnix != int64(2*time.Minute) ||
		status.LagSeconds != int64((3*time.Minute)/time.Second) ||
		status.NextRunUnix <= 0 ||
		status.WindowsProcessed != 3 ||
		status.PointsWritten != 9 {
		t.Fatalf("status = %#v, want per-policy details", status)
	}
	metrics := eng.MetricsSnapshot()
	names := metricNameSet(metrics)
	for _, name := range []string{
		"mts_downsample_policy_lag_seconds",
		"mts_downsample_policy_last_watermark_unix",
		"mts_downsample_policy_windows_processed_total",
		"mts_downsample_policy_points_written_total",
	} {
		if _, ok := names[name]; !ok {
			t.Fatalf("metric %s missing in %#v", name, names)
		}
		if !hasMetricLabel(metrics, name, "policy", "cpu_1m") {
			t.Fatalf("metric %s missing policy label in %#v", name, metrics)
		}
	}
}

func hasMetricLabel(
	metrics []observability.Metric,
	name string,
	label string,
	value string,
) bool {
	for _, metric := range metrics {
		if metric.Name == name && metric.Labels[label] == value {
			return true
		}
	}
	return false
}

func healthCheckStatus(health HealthSnapshot, name string, status string) bool {
	for _, check := range health.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}
