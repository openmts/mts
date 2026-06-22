package engine

import (
	"context"
	"math"
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

func TestDownsampleAggregateStateValueBranches(t *testing.T) {
	tests := []struct {
		name      string
		function  string
		samples   []downsampleSample
		wantType  model.FieldType
		wantInt   int64
		wantFloat float64
	}{
		{
			name:     "sum preserves int type",
			function: "sum",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Int64Value(1)},
				{timestamp: int64(time.Second), value: model.Int64Value(2)},
				{timestamp: int64(2 * time.Second), value: model.Int64Value(3)},
			},
			wantType: model.FieldInt64,
			wantInt:  6,
		},
		{
			name:     "sum preserves float type",
			function: "sum",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(1.25)},
				{timestamp: int64(time.Second), value: model.Float64Value(2.75)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 4,
		},
		{
			name:     "spread preserves int type",
			function: "spread",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Int64Value(4)},
				{timestamp: int64(time.Second), value: model.Int64Value(1)},
			},
			wantType: model.FieldInt64,
			wantInt:  3,
		},
		{
			name:     "spread preserves float type",
			function: "spread",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(4.5)},
				{timestamp: int64(time.Second), value: model.Float64Value(1.5)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 3,
		},
		{
			name:     "stdvar returns variance",
			function: "stdvar",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(1)},
				{timestamp: int64(time.Second), value: model.Float64Value(3)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 1,
		},
		{
			name:     "increase handles counter reset",
			function: "increase",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(5)},
				{timestamp: int64(time.Second), value: model.Float64Value(7)},
				{timestamp: int64(2 * time.Second), value: model.Float64Value(2)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 4,
		},
		{
			name:     "delta preserves int type",
			function: "delta",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Int64Value(2)},
				{timestamp: int64(time.Second), value: model.Int64Value(5)},
			},
			wantType: model.FieldInt64,
			wantInt:  3,
		},
		{
			name:     "derivative divides delta by seconds",
			function: "derivative",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(2)},
				{timestamp: int64(3 * time.Second), value: model.Float64Value(8)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 2,
		},
		{
			name:     "median averages even sample count",
			function: "median",
			samples: []downsampleSample{
				{timestamp: 0, value: model.Float64Value(9)},
				{timestamp: int64(time.Second), value: model.Float64Value(1)},
				{timestamp: int64(2 * time.Second), value: model.Float64Value(5)},
				{timestamp: int64(3 * time.Second), value: model.Float64Value(3)},
			},
			wantType:  model.FieldFloat64,
			wantFloat: 4,
		},
		{
			name:     "mode compares strings",
			function: "mode",
			samples: []downsampleSample{
				{timestamp: 0, value: model.StringValue("z")},
				{timestamp: int64(time.Second), value: model.StringValue("a")},
				{timestamp: int64(2 * time.Second), value: model.StringValue("a")},
			},
			wantType: model.FieldString,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := downsampleAggregateState{fn: tt.function}
			for _, sample := range tt.samples {
				if err := state.add(sample.timestamp, sample.value); err != nil {
					t.Fatalf("add() error = %v", err)
				}
			}
			got, err := state.value()
			if err != nil {
				t.Fatalf("value() error = %v", err)
			}
			if got.Type != tt.wantType {
				t.Fatalf("value type = %v, want %v", got.Type, tt.wantType)
			}
			if tt.wantType == model.FieldInt64 && got.Int64 != tt.wantInt {
				t.Fatalf("value = %#v, want int %d", got, tt.wantInt)
			}
			if tt.wantType == model.FieldFloat64 && math.Abs(got.Float64-tt.wantFloat) > 1e-9 {
				t.Fatalf("value = %#v, want float %f", got, tt.wantFloat)
			}
			if tt.wantType == model.FieldString && got.String != "a" {
				t.Fatalf("value = %#v, want string a", got)
			}
		})
	}
}

func TestDownsampleAggregateStateRejectsInvalidValues(t *testing.T) {
	sum := downsampleAggregateState{fn: "sum"}
	if err := sum.add(0, model.StringValue("bad")); err == nil {
		t.Fatal("add(non-numeric sum) error = nil, want error")
	}
	avg := downsampleAggregateState{fn: "avg"}
	if err := avg.add(0, model.Int64Value(1)); err != nil {
		t.Fatalf("add(first avg) error = %v", err)
	}
	if err := avg.add(1, model.Float64Value(2)); err == nil {
		t.Fatal("add(mixed avg) error = nil, want error")
	}
	rate := downsampleAggregateState{fn: "rate"}
	if err := rate.add(0, model.Float64Value(1)); err != nil {
		t.Fatalf("add(rate) error = %v", err)
	}
	if _, err := rate.value(); err == nil {
		t.Fatal("rate.value(one sample) error = nil, want error")
	}
	derivative := downsampleAggregateState{fn: "derivative"}
	if err := derivative.add(0, model.Float64Value(1)); err != nil {
		t.Fatalf("add(first derivative) error = %v", err)
	}
	if err := derivative.add(0, model.Float64Value(2)); err != nil {
		t.Fatalf("add(second derivative) error = %v", err)
	}
	if _, err := derivative.value(); err == nil {
		t.Fatal("derivative.value(equal timestamps) error = nil, want error")
	}
	unknown := downsampleAggregateState{
		fn: "unknown",
		series: []downsampleSample{
			{timestamp: 0, value: model.Float64Value(1)},
			{timestamp: int64(time.Second), value: model.Float64Value(2)},
		},
	}
	if _, err := unknown.seriesValue(); err == nil {
		t.Fatal("seriesValue(unknown) error = nil, want error")
	}
	if _, err := downsampleRate([]downsampleSample{
		{timestamp: 0, value: model.Float64Value(1)},
		{timestamp: 0, value: model.Float64Value(2)},
	}, false); err == nil {
		t.Fatal("downsampleRate(equal timestamps) error = nil, want error")
	}
	if key := downsampleFieldValueKey(model.BoolValue(true)); key != "b:true" {
		t.Fatalf("bool key = %q, want b:true", key)
	}
	if key := downsampleFieldValueKey(model.FieldValue{}); key != "unknown" {
		t.Fatalf("unknown key = %q, want unknown", key)
	}
	if tags := downsampleGroupTags(map[string]string{"host": "a"}, nil); tags != nil {
		t.Fatalf("downsampleGroupTags(no names) = %#v, want nil", tags)
	}
	if tags := cloneDownsampleTags(nil); tags != nil {
		t.Fatalf("cloneDownsampleTags(nil) = %#v, want nil", tags)
	}
	targetTags := downsampleTargetTags(nil, model.DownsamplePolicy{
		Name:          "p",
		PolicyTagName: "policy",
	})
	if targetTags["policy"] != "p" {
		t.Fatalf("downsampleTargetTags(nil) = %#v, want policy tag", targetTags)
	}
}

func TestDownsampleColumnsToPointsBuildsSortedTargetPoints(t *testing.T) {
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
		Enabled:         true,
	})
	points, err := downsampleColumnsToPoints(policy, 0, int64(time.Minute), []model.ColumnSeries{
		{
			FieldName:  "avg(usage)",
			Tags:       map[string]string{"host": "b"},
			Timestamps: []int64{0, int64(time.Minute)},
			Values:     []model.FieldValue{model.Float64Value(2), model.Float64Value(20)},
		},
		{
			FieldName:  "max(usage)",
			Tags:       map[string]string{"host": "b"},
			Timestamps: []int64{0},
			Values:     []model.FieldValue{model.Float64Value(3)},
		},
		{
			FieldName:  "avg(usage)",
			Tags:       map[string]string{"host": "a"},
			Timestamps: []int64{0},
			Values:     []model.FieldValue{model.Float64Value(1)},
		},
	})
	if err != nil {
		t.Fatalf("downsampleColumnsToPoints() error = %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("points = %#v, want two sorted target points", points)
	}
	if points[0].Tags["host"] != "a" || points[1].Tags["host"] != "b" {
		t.Fatalf("points = %#v, want host a before b", points)
	}
	if points[1].Fields["avg_usage"].Float64 != 2 ||
		points[1].Fields["max_usage"].Float64 != 3 ||
		points[1].Tags[defaultDownsamplePolicyTagName] != "cpu_1m" {
		t.Fatalf("point b = %#v, want avg/max fields and policy tag", points[1])
	}
}

func TestDownsampleRangeWindowsRejectsInvalidBoundaries(t *testing.T) {
	policy := testDownsamplePolicy()
	if _, _, _, err := downsampleRangeWindows(policy, model.DownsampleWatermark{}, -1, 1, false); err == nil {
		t.Fatal("downsampleRangeWindows(negative) error = nil, want error")
	}
	if _, _, _, err := downsampleRangeWindows(policy, model.DownsampleWatermark{}, 2, 1, false); err == nil {
		t.Fatal("downsampleRangeWindows(backward) error = nil, want error")
	}
	windows, alignedStart, alignedEnd, err := downsampleRangeWindows(
		policy,
		model.DownsampleWatermark{},
		int64(time.Second),
		int64(2*time.Second),
		false,
	)
	if err != nil {
		t.Fatalf("downsampleRangeWindows(short aligned range) error = %v", err)
	}
	if len(windows) != 0 || alignedStart != 0 || alignedEnd != 0 {
		t.Fatalf("windows=%#v aligned=%d/%d, want empty 0/0", windows, alignedStart, alignedEnd)
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
