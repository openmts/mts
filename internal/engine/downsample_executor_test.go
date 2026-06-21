package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestDownsampleExecutorWritesAggregatedPoints(t *testing.T) {
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
	for minute := int64(0); minute < 5; minute++ {
		point := model.Point{
			Database:        "metrics",
			RetentionPolicy: "autogen",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "a", "region": "east"},
			Timestamp:       minute * int64(time.Minute),
			Fields: map[string]model.FieldValue{
				"usage": model.Float64Value(float64(minute)),
			},
		}
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{}); err != nil {
			t.Fatalf("Write(raw) error = %v", err)
		}
	}
	policy := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name:              "cpu_5m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_5m",
		TargetMeasurement: "cpu",
		Interval:          5 * time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		GroupByTags:     []string{"host"},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        5 * time.Minute,
		Enabled:         true,
	})
	result, err := eng.runDownsampleWindow(ctx, policy, 0, int64(5*time.Minute))
	if err != nil {
		t.Fatalf("runDownsampleWindow() error = %v", err)
	}
	if result.PointsWritten != 1 {
		t.Fatalf("PointsWritten = %d, want 1", result.PointsWritten)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_5m",
		Measurement:     "cpu",
		Fields:          []string{"avg_usage"},
		StartTime:       0,
		EndTime:         int64(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v, want one row", rows)
	}
	if rows[0].Tags["host"] != "a" ||
		rows[0].Tags[defaultDownsamplePolicyTagName] != "cpu_5m" ||
		len(rows[0].Tags) != 2 {
		t.Fatalf("target tags = %#v, want host and policy tag", rows[0].Tags)
	}
	if rows[0].Fields["avg_usage"].Float64 != 2 {
		t.Fatalf("avg_usage = %#v, want 2", rows[0].Fields["avg_usage"])
	}
}

func TestDownsampleExecutorWritesCompletePointsWithSmallBatch(t *testing.T) {
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
	for host, usage := range map[string]float64{"a": 1, "b": 2} {
		point := model.Point{
			Database:        "metrics",
			RetentionPolicy: "autogen",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": host},
			Timestamp:       0,
			Fields: map[string]model.FieldValue{
				"usage": model.Float64Value(usage),
			},
		}
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{}); err != nil {
			t.Fatalf("Write(raw) error = %v", err)
		}
	}
	policy := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "avg", Field: "usage"},
			{Function: "max", Field: "usage"},
		},
		GroupByTags:     []string{"host"},
		RefreshInterval: time.Minute,
		BatchSize:       1,
		Enabled:         true,
	})
	result, err := eng.runDownsampleWindow(ctx, policy, 0, int64(time.Minute))
	if err != nil {
		t.Fatalf("runDownsampleWindow() error = %v", err)
	}
	if result.PointsWritten != 2 {
		t.Fatalf("PointsWritten = %d, want two grouped points", result.PointsWritten)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         int64(time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two target rows", rows)
	}
	for _, row := range rows {
		if row.Fields["avg_usage"].Type == 0 || row.Fields["max_usage"].Type == 0 {
			t.Fatalf("row = %#v, want complete avg/max fields", row)
		}
	}
}

func TestDownsampleExecutorSupportsCommercialAggregateFunctions(t *testing.T) {
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
		downsampleFunctionPoint(0, 1),
		downsampleFunctionPoint(int64(time.Minute), 3),
		downsampleFunctionPoint(int64(90*time.Second), 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write(raw) error = %v", err)
	}
	policy := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name:              "cpu_2m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_2m",
		TargetMeasurement: "cpu",
		Interval:          2 * time.Minute,
		Functions: []model.DownsampleFunction{
			{Function: "rate", Field: "usage"},
			{Function: "difference", Field: "usage"},
			{Function: "mode", Field: "usage"},
			{Function: "stddev", Field: "usage"},
		},
		GroupByTags:     []string{"host"},
		RefreshInterval: time.Minute,
		Enabled:         true,
	})
	result, err := eng.runDownsampleWindow(ctx, policy, 0, int64(2*time.Minute))
	if err != nil {
		t.Fatalf("runDownsampleWindow() error = %v", err)
	}
	if result.PointsWritten != 1 {
		t.Fatalf("PointsWritten = %d, want one row", result.PointsWritten)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_2m",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         int64(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v, want one target row", rows)
	}
	fields := rows[0].Fields
	if fields["difference_usage"].Float64 != 2 {
		t.Fatalf("difference_usage = %#v, want 2", fields["difference_usage"])
	}
	if fields["mode_usage"].Float64 != 3 {
		t.Fatalf("mode_usage = %#v, want 3", fields["mode_usage"])
	}
	if fields["rate_usage"].Float64 <= 0 || fields["stddev_usage"].Float64 <= 0 {
		t.Fatalf("fields = %#v, want positive rate and stddev", fields)
	}
}

func downsampleFunctionPoint(timestamp int64, usage float64) model.Point {
	return model.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       timestamp,
		Fields:          map[string]model.FieldValue{"usage": model.Float64Value(usage)},
	}
}

func TestDownsampleExecutorSkipsEmptyWindow(t *testing.T) {
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
	policy := mustNormalizeDownsamplePolicyForTest(t, model.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []model.DownsampleFunction{{
			Function: "count",
			Field:    "usage",
		}},
		GroupByTags:     []string{"host"},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        time.Minute,
		Enabled:         true,
	})
	result, err := eng.runDownsampleWindow(ctx, policy, 0, int64(time.Minute))
	if err != nil {
		t.Fatalf("runDownsampleWindow(empty) error = %v", err)
	}
	if result.PointsWritten != 0 {
		t.Fatalf("PointsWritten = %d, want 0", result.PointsWritten)
	}
}

func mustNormalizeDownsamplePolicyForTest(
	t *testing.T,
	policy model.DownsamplePolicy,
) model.DownsamplePolicy {
	t.Helper()
	normalized, err := normalizeDownsamplePolicy(policy)
	if err != nil {
		t.Fatalf("normalizeDownsamplePolicy() error = %v", err)
	}
	return normalized
}
