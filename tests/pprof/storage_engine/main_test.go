package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mts "codeberg.org/mts/mts"
)

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"storage_engine", "-points", "20", "-series", "5", "-query-repeat", "1"}
	defer func() {
		os.Args = oldArgs
	}()
	main()
}

func TestRunWithProfiles(t *testing.T) {
	root := t.TempDir()
	cpuProfile := filepath.Join(root, "cpu.prof")
	memProfile := filepath.Join(root, "mem.prof")
	err := run([]string{
		"-data-dir", filepath.Join(root, "data"),
		"-points", "200",
		"-series", "10",
		"-query-repeat", "2",
		"-cpu-profile", cpuProfile,
		"-mem-profile", memProfile,
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	assertNonEmptyFile(t, cpuProfile)
	assertNonEmptyFile(t, memProfile)
}

func TestRunWithTemporaryDataDir(t *testing.T) {
	err := run([]string{
		"-points", "20",
		"-series", "5",
		"-query-repeat", "1",
	})
	if err != nil {
		t.Fatalf("run(temp) error = %v", err)
	}
}

func TestRunModes(t *testing.T) {
	for _, mode := range []string{"write", "query", "compact", "replay"} {
		t.Run(mode, func(t *testing.T) {
			err := run([]string{"-mode", mode, "-points", "200", "-series", "10", "-query-repeat", "2"})
			if err != nil {
				t.Fatalf("run(%s) error = %v", mode, err)
			}
		})
	}
}

func TestRunRejectsBadInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "bad flag", args: []string{"-unknown"}},
		{name: "invalid config", args: []string{"-points", "1", "-series", "2"}},
		{name: "bad field layout", args: []string{"-field-layout", "bad", "-points", "1", "-series", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args); err == nil {
				t.Fatal("run() error = nil, want error")
			}
		})
	}
}

func TestRunRejectsRuntimeErrors(t *testing.T) {
	root := t.TempDir()
	parentFile := filepath.Join(root, "parent")
	if err := os.WriteFile(parentFile, []byte("not a dir"), 0600); err != nil {
		t.Fatalf("WriteFile(parent) error = %v", err)
	}
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "data dir parent is file",
			args: []string{"-data-dir", filepath.Join(parentFile, "data"), "-points", "1", "-series", "1"},
		},
		{
			name: "cpu profile parent is file",
			args: []string{"-cpu-profile", filepath.Join(parentFile, "cpu.prof"), "-points", "1", "-series", "1"},
		},
		{
			name: "mem profile parent is file",
			args: []string{"-mem-profile", filepath.Join(parentFile, "mem.prof"), "-points", "1", "-series", "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args); err == nil {
				t.Fatal("run() error = nil, want error")
			}
		})
	}
}

func TestParseConfigRejectsBadFlag(t *testing.T) {
	if _, err := parseConfig([]string{"-unknown"}); err == nil {
		t.Fatal("parseConfig(bad flag) error = nil, want error")
	}
}

func TestPrepareDataDirRejectsInvalidPath(t *testing.T) {
	if _, _, err := prepareDataDir("bad\x00path"); err == nil {
		t.Fatal("prepareDataDir(invalid) error = nil, want error")
	}
}

func TestStartCPUProfileRejectsInvalidPath(t *testing.T) {
	if _, err := startCPUProfile("bad\x00path"); err == nil {
		t.Fatal("startCPUProfile(invalid) error = nil, want error")
	}
}

func TestStartCPUProfileRejectsConcurrentProfile(t *testing.T) {
	root := t.TempDir()
	stop, err := startCPUProfile(filepath.Join(root, "first.prof"))
	if err != nil {
		t.Fatalf("startCPUProfile(first) error = %v", err)
	}
	if _, err := startCPUProfile(filepath.Join(root, "second.prof")); err == nil {
		stopErr := stop()
		t.Fatalf("startCPUProfile(second) error = nil, want error; stop = %v", stopErr)
	}
	if err := stop(); err != nil {
		t.Fatalf("stop CPU profile error = %v", err)
	}
}

func TestWritePointsExactBatch(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2048,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writePoints(ctx, eng, config{points: 1024, series: 16}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("writePoints() error = %v close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWritePointsSyncedExactBatch(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2048,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writePointsSynced(ctx, eng, config{points: 1024, series: 16}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("writePointsSynced() error = %v close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWide10WorkloadPointHasExpectedFields(t *testing.T) {
	point := workloadPoint(7, 3, fieldLayoutWide10)
	if len(point.Fields) != 10 {
		t.Fatalf("field count = %d, want 10", len(point.Fields))
	}
	counts := countFieldTypes(point.Fields)
	if counts[mts.FieldFloat64] != 5 {
		t.Fatalf("float field count = %d, want 5", counts[mts.FieldFloat64])
	}
	if counts[mts.FieldInt64] != 3 {
		t.Fatalf("int field count = %d, want 3", counts[mts.FieldInt64])
	}
	if counts[mts.FieldString] != 1 {
		t.Fatalf("string field count = %d, want 1", counts[mts.FieldString])
	}
	if counts[mts.FieldBool] != 1 {
		t.Fatalf("bool field count = %d, want 1", counts[mts.FieldBool])
	}
}

func TestWritePointsRejectsClosedEngine(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := writePoints(ctx, eng, config{points: 1, series: 1}); err == nil {
		t.Fatal("writePoints(closed) error = nil, want error")
	}
}

func TestRunWorkloadRejectsClosedEngine(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := runWorkload(ctx, eng, config{points: 1, series: 1}); err == nil {
		t.Fatal("runWorkload(closed) error = nil, want error")
	}
}

func TestWorkloadModesRejectRuntimeErrors(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		cfg  config
	}{
		{name: "unsupported mode", cfg: config{mode: "bad", points: 1, series: 1}},
		{name: "replay missing dir", cfg: config{mode: "replay", points: 1, series: 1}},
		{name: "query closed engine", cfg: config{mode: "query", points: 1, series: 1}},
		{name: "compact closed engine", cfg: config{mode: "compact", points: 1, series: 1}},
		{name: "write closed engine", cfg: config{mode: "write", points: 1, series: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if err := eng.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if err := runWorkloadWithDir(ctx, eng, tt.cfg, ""); err == nil {
				t.Fatal("runWorkloadWithDir() error = nil, want error")
			}
		})
	}
}

func TestRunWorkloadRejectsMissingQueriedSeries(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 16,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = runWorkload(ctx, eng, config{points: 1, series: 2, queryRepeat: 2})
	if closeErr := eng.Close(ctx); closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
	if err == nil {
		t.Fatal("runWorkload(missing queried series) error = nil, want error")
	}
}

func TestQueryRowsRejectsEmptyResult(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = queryRows(ctx, eng, config{points: 10, series: 1, queryRepeat: 1})
	if closeErr := eng.Close(ctx); closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
	if err == nil {
		t.Fatal("queryRows(empty) error = nil, want error")
	}
}

func TestQueryRowsRejectsClosedEngine(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := queryRows(ctx, eng, config{points: 1, series: 1, queryRepeat: 1}); err == nil {
		t.Fatal("queryRows(closed) error = nil, want error")
	}
}

func TestWritePointsSyncedRejectsClosedEngine(t *testing.T) {
	ctx := context.Background()
	for _, points := range []int{1, 1024} {
		t.Run(fmt.Sprintf("points=%d", points), func(t *testing.T) {
			eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if err := eng.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if err := writePointsSynced(ctx, eng, config{points: points, series: 1}); err == nil {
				t.Fatal("writePointsSynced(closed) error = nil, want error")
			}
		})
	}
}

func TestReplayWorkloadRejectsReopenError(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := replayWorkload(ctx, eng, config{points: 1, series: 1}, "bad\x00path"); err == nil {
		t.Fatal("replayWorkload(reopen) error = nil, want error")
	}
}

func TestReplayWorkloadRejectsWriteError(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := replayWorkload(ctx, eng, config{points: 1, series: 1}, t.TempDir()); err == nil {
		t.Fatal("replayWorkload(write error) error = nil, want error")
	}
}

func TestReplayWorkloadRejectsMissingQueriedSeries(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = replayWorkload(ctx, eng, config{points: 1, series: 2, queryRepeat: 2}, dir)
	if err == nil {
		t.Fatal("replayWorkload(missing queried series) error = nil, want error")
	}
}

func TestWriteMemProfileEmptyPath(t *testing.T) {
	if err := writeMemProfile(""); err != nil {
		t.Fatalf("writeMemProfile(empty) error = %v", err)
	}
}

func TestWriteMemProfileRejectsInvalidPath(t *testing.T) {
	if err := writeMemProfile("bad\x00path"); err == nil {
		t.Fatal("writeMemProfile(invalid) error = nil, want error")
	}
}

func TestCreateProfileFileParentIsFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "profile-parent")
	if err := os.WriteFile(parent, []byte("not a dir"), 0600); err != nil {
		t.Fatalf("WriteFile(parent) error = %v", err)
	}
	if _, err := createProfileFile(filepath.Join(parent, "cpu.prof")); err == nil {
		t.Fatal("createProfileFile(parent file) error = nil, want error")
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  config
	}{
		{name: "bad mode", cfg: config{mode: "bad", points: 1, series: 1}},
		{name: "zero points", cfg: config{mode: "query", points: 0, series: 1}},
		{name: "zero series", cfg: config{mode: "query", points: 1, series: 0}},
		{name: "series greater than points", cfg: config{mode: "query", points: 1, series: 2}},
		{name: "negative query repeat", cfg: config{mode: "query", points: 1, series: 1, queryRepeat: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateConfig(tt.cfg); err == nil {
				t.Fatal("validateConfig() error = nil, want error")
			}
		})
	}
}

func TestCreateProfileFileRejectsInvalidPath(t *testing.T) {
	if _, err := createProfileFile("bad\x00path"); err == nil {
		t.Fatal("createProfileFile(invalid) error = nil, want error")
	}
}

func assertNonEmptyFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("file %s is empty", path)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file %s mode = %v, want 0600", path, info.Mode().Perm())
	}
}

func countFieldTypes(fields map[string]mts.FieldValue) map[mts.FieldType]int {
	counts := make(map[mts.FieldType]int)
	for _, value := range fields {
		counts[value.Type]++
	}
	return counts
}
