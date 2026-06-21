package mts_test

import (
	"context"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestDownsamplePolicyPublicAPIPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := mts.Open(ctx, mts.Options{Path: dir})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	policy := mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "default",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{{
			Function: "mean",
			Field:    "usage",
		}},
		GroupByTags:        []string{"host"},
		Delay:              time.Minute,
		RefreshInterval:    time.Minute,
		Lookback:           3 * time.Minute,
		InitialStartTime:   int64(2 * time.Minute),
		RunTimeout:         2 * time.Minute,
		BatchSize:          17,
		CheckpointInterval: 3,
		PolicyTagName:      "policy",
		Enabled:            true,
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	if err := eng.DisableDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("DisableDownsamplePolicy() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := mts.Open(ctx, mts.Options{Path: dir})
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(ctx); err != nil {
			t.Fatalf("Close(reopened) error = %v", err)
		}
	}()
	policies, err := reopened.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Enabled {
		t.Fatalf("policies = %#v, want one disabled policy", policies)
	}
	if policies[0].Functions[0].Function != "avg" ||
		policies[0].Functions[0].As != "avg_usage" {
		t.Fatalf("function = %#v, want normalized avg_usage", policies[0].Functions[0])
	}
	if policies[0].InitialStartTime != int64(2*time.Minute) ||
		policies[0].RunTimeout != 2*time.Minute ||
		policies[0].BatchSize != 17 ||
		policies[0].CheckpointInterval != 3 ||
		policies[0].PolicyTagName != "policy" {
		t.Fatalf("policy controls = %#v, want persisted commercial controls", policies[0])
	}
}

func TestDownsamplePolicyPublicAPIDropsPolicy(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.CreateDownsamplePolicy(ctx, mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "default",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		Delay:           time.Minute,
		RefreshInterval: time.Minute,
		Lookback:        time.Minute,
		Enabled:         true,
	}); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	if err := eng.DropDownsamplePolicy(ctx, "cpu_1m"); err != nil {
		t.Fatalf("DropDownsamplePolicy() error = %v", err)
	}
	policies, err := eng.ListDownsamplePolicies(ctx)
	if err != nil {
		t.Fatalf("ListDownsamplePolicies() error = %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("policies = %#v, want empty", policies)
	}
}

func TestDownsamplePolicyPublicAPIRunsPolicy(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.Write(ctx, []mts.Point{{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       0,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(4),
		},
	}}, mts.WriteOptions{}); err != nil {
		t.Fatalf("Write(raw) error = %v", err)
	}
	policy := mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		GroupByTags:     []string{"host"},
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
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", time.Unix(0, int64(time.Minute)))
	if err != nil {
		t.Fatalf("RunDownsamplePolicy() error = %v", err)
	}
	if result.WindowsProcessed != 1 || result.PointsWritten != 1 ||
		result.CompletedUntilUnix != int64(time.Minute) {
		t.Fatalf("result = %#v, want one completed window", result)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         int64(time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryRows(target) error = %v", err)
	}
	if len(rows) != 1 || rows[0].Fields["avg_usage"].Float64 != 4 {
		t.Fatalf("rows = %#v, want avg_usage=4", rows)
	}
}
