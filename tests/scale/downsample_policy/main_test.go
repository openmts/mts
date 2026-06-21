package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mts "github.com/openmts/mts"
)

func TestRunSmoke(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.json")
	if err := run([]string{"-points", "1000", "-series", "10", "-out", out}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile(report) error = %v", err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(report) error = %v", err)
	}
	if got.Points != 1000 || got.Series != 10 ||
		got.PolicyCount != 1 || got.BatchSize <= 0 ||
		got.CheckpointInterval <= 0 || got.RunTimeoutNanos <= 0 ||
		got.StatusCount != got.PolicyCount ||
		got.WindowsProcessed == 0 || got.PointsWritten == 0 ||
		got.WriteDurationNanos <= 0 || got.DownsampleDurationNanos <= 0 ||
		got.QueryDurationNanos <= 0 || got.QueryRows == 0 ||
		got.WriteThroughput <= 0 || got.DownsampleThroughput <= 0 ||
		got.QueryThroughput <= 0 || got.WriteRSSPeakBytes == 0 ||
		got.DownsampleRSSPeakBytes == 0 || got.QueryRSSPeakBytes == 0 ||
		got.HeapAllocBytes == 0 || got.HeapInuseBytes == 0 ||
		got.DiskBytes == 0 || got.SSTableCount < 0 ||
		!got.Verify || !got.Verified {
		t.Fatalf("report = %#v, want populated downsample report", got)
	}
}

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	out := filepath.Join(t.TempDir(), "report.json")
	os.Args = []string{"downsample_policy", "-points", "100", "-series", "10", "-query-limit", "10", "-out", out}
	main()
	os.Args = oldArgs
}

func TestParseConfigRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"-points", "0"},
		{"-series", "0"},
		{"-points", "10", "-series", "11"},
		{"-query-limit", "0"},
		{"-policy-count", "0"},
		{"-batch-size", "0"},
		{"-checkpoint-interval", "0"},
		{"-run-timeout", "0s"},
		{"-initial-start", "-1"},
	} {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%v) error = nil, want error", args)
		}
	}
}

func TestParseConfigAcceptsVerifyFalse(t *testing.T) {
	cfg, err := parseConfig([]string{
		"-points", "10",
		"-series", "2",
		"-verify=false",
		"-policy-count", "2",
		"-batch-size", "7",
		"-checkpoint-interval", "3",
		"-run-timeout", "2m",
		"-initial-start", "100",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.verify {
		t.Fatal("verify = true, want false")
	}
	if cfg.queryLimit != defaultQueryLimit {
		t.Fatalf("queryLimit = %d, want %d", cfg.queryLimit, defaultQueryLimit)
	}
	if cfg.policyCount != 2 || cfg.batchSize != 7 ||
		cfg.checkpointInterval != 3 || cfg.runTimeout.String() != "2m0s" ||
		cfg.initialStart != 100 {
		t.Fatalf("cfg = %#v, want parsed commercial knobs", cfg)
	}
}

func TestRunWorkloadRejectsInvalidPath(t *testing.T) {
	_, err := runWorkload(context.Background(), "bad\x00path", config{points: 10, series: 1})
	if err == nil {
		t.Fatal("runWorkload(invalid path) error = nil, want error")
	}
}

func TestVerifyRowsRejectsInvalidRows(t *testing.T) {
	cfg := config{points: 100, series: 10, queryLimit: 10, verify: true}
	cases := map[string][]mts.Row{
		"empty": {},
		"bad_host": {{
			Tags: map[string]string{"host": "bad"},
			Fields: map[string]mts.FieldValue{
				"avg_usage":   mts.Float64Value(0),
				"min_usage":   mts.Float64Value(0),
				"max_usage":   mts.Float64Value(0),
				"last_usage":  mts.Float64Value(0),
				"count_usage": mts.Int64Value(1),
			},
		}},
		"bad_value": {{
			Tags:      map[string]string{"host": "h0"},
			Timestamp: 0,
			Fields: map[string]mts.FieldValue{
				"avg_usage":   mts.Float64Value(99),
				"min_usage":   mts.Float64Value(0),
				"max_usage":   mts.Float64Value(90),
				"last_usage":  mts.Float64Value(90),
				"count_usage": mts.Int64Value(10),
			},
		}},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			if err := verifyRows(rows, cfg); err == nil {
				t.Fatal("verifyRows() error = nil, want error")
			}
		})
	}
}
