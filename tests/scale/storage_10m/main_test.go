package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/mts/mts/internal/faultinject"
	"codeberg.org/mts/mts/internal/storagefs"
)

func TestRunSmoke(t *testing.T) {
	if err := run([]string{"-points", "100000"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"storage_10m", "-points", "8", "-batch-size", "3"}
	main()
	os.Args = oldArgs
}

func TestParseConfigRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"-points", "0"},
		{"-batch-size", "0"},
		{"-mode", "bad"},
	} {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%v) error = nil, want error", args)
		}
	}
}

func TestPrepareDirExplicitAndInvalidPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	got, cleanup, err := prepareDir(dir)
	if err != nil {
		t.Fatalf("prepareDir(explicit) error = %v", err)
	}
	if got != dir {
		t.Fatalf("prepareDir() dir = %q, want %q", got, dir)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, _, err := prepareDir(filepath.Join(filePath, "child")); err == nil {
		t.Fatal("prepareDir(invalid) error = nil, want error")
	}
}

func TestRunWorkloadModes(t *testing.T) {
	for _, mode := range []string{"write", "query", "compact", "restart"} {
		t.Run(mode, func(t *testing.T) {
			rows, err := runWorkload(t.TempDir(), config{mode: mode, points: 16, batchSize: 5})
			if err != nil {
				t.Fatalf("runWorkload(%s) error = %v", mode, err)
			}
			if mode == "write" {
				if rows != 0 {
					t.Fatalf("runWorkload(write) rows = %d, want 0", rows)
				}
				return
			}
			if rows != 16 {
				t.Fatalf("runWorkload(%s) rows = %d, want 16", mode, rows)
			}
		})
	}
}

func TestReportIncludesAmplificationAndLevelDistribution(t *testing.T) {
	out := report{
		LevelDistribution:  map[int]int{1: 2},
		ReadAmplification:  1.5,
		WriteAmplification: 2.5,
		SpaceAmplification: 0.75,
		QueryLatencyNanos:  123,
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(data)
	for _, key := range []string{
		"level_distribution",
		"read_amplification",
		"write_amplification",
		"space_amplification",
		"query_latency_nanos",
		"compaction_stats",
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("report json %s missing %s", text, key)
		}
	}
}

func TestRunWorkloadRejectsInvalidDirectory(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := runWorkload(filepath.Join(filePath, "child"), config{mode: "write", points: 1, batchSize: 1}); err == nil {
		t.Fatal("runWorkload(invalid dir) error = nil, want error")
	}
}

func TestRunWorkloadPropagatesStorageFaults(t *testing.T) {
	tests := []struct {
		name string
		mode string
		op   faultinject.Operation
	}{
		{name: "write", mode: "write", op: faultinject.OpWrite},
		{name: "flush", mode: "write", op: faultinject.OpRename},
		{name: "restart", mode: "restart", op: faultinject.OpStat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := faultinject.NewFS()
			fs.FailNext(tt.op, os.ErrPermission)
			restore := storagefs.SetFaultController(fs)
			_, err := runWorkload(t.TempDir(), config{mode: tt.mode, points: 8, batchSize: 4})
			restore()
			if err == nil {
				t.Fatalf("runWorkload(%s fault %s) error = nil, want error", tt.mode, tt.op)
			}
		})
	}
}

func TestRunRejectsInvalidConfigAndDirectory(t *testing.T) {
	if err := run([]string{"-mode", "bad"}); err == nil {
		t.Fatal("run(invalid mode) error = nil, want error")
	}
	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := run([]string{"-data-dir", filepath.Join(filePath, "child"), "-points", "1"}); err == nil {
		t.Fatal("run(invalid dir) error = nil, want error")
	}
}

func TestCompareBaseline(t *testing.T) {
	dir := t.TempDir()
	base := report{Duration: 100, DataBytes: 100, HeapAlloc: 100}
	path := filepath.Join(dir, "baseline.json")
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := config{baseline: path, maxRegressionPercent: 10}
	if err := compareBaseline(cfg, report{Duration: 105, DataBytes: 100, HeapAlloc: 100}); err != nil {
		t.Fatalf("compareBaseline(within limit) error = %v", err)
	}
	if err := compareBaseline(cfg, report{Duration: 120, DataBytes: 100, HeapAlloc: 100}); err == nil {
		t.Fatal("compareBaseline(duration regression) error = nil, want error")
	}
	if err := compareBaseline(cfg, report{Duration: 100, DataBytes: 120, HeapAlloc: 100}); err == nil {
		t.Fatal("compareBaseline(data regression) error = nil, want error")
	}
	if err := compareBaseline(cfg, report{Duration: 100, DataBytes: 100, HeapAlloc: 120}); err == nil {
		t.Fatal("compareBaseline(heap regression) error = nil, want error")
	}
	if err := compareBaseline(config{baseline: filepath.Join(dir, "missing")}, report{}); err == nil {
		t.Fatal("compareBaseline(missing) error = nil, want error")
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(invalid) error = %v", err)
	}
	if err := compareBaseline(cfg, report{}); err == nil {
		t.Fatal("compareBaseline(invalid json) error = nil, want error")
	}
}

func TestRegressionPercent(t *testing.T) {
	if got := regressionPercent(120, 100); got != 20 {
		t.Fatalf("regressionPercent() = %v, want 20", got)
	}
	if got := regressionPercent(80, 100); got != 0 {
		t.Fatalf("regressionPercent(no regression) = %v, want 0", got)
	}
	if got := regressionPercent(80, 0); got != 0 {
		t.Fatalf("regressionPercent(zero baseline) = %v, want 0", got)
	}
}
