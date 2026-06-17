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

func TestRunWithPrebuiltPoints(t *testing.T) {
	err := run([]string{
		"-prebuild-points",
		"-field-layout", fieldLayoutWide10,
		"-points", "100",
		"-series", "10",
		"-query-repeat", "1",
	})
	if err != nil {
		t.Fatalf("run(prebuild) error = %v", err)
	}
	cfg := config{points: 10, series: 2, fieldLayout: fieldLayoutWide10}
	prebuilt := buildWorkloadPoints(cfg)
	cfg.prebuilt = prebuilt
	if got := workloadPointAt(cfg, 3); len(got.Fields) != 10 {
		t.Fatalf("prebuilt field count = %d, want 10", len(got.Fields))
	}
}

func TestRunWithStorageParameters(t *testing.T) {
	root := t.TempDir()
	err := run([]string{
		"-data-dir", filepath.Join(root, "data"),
		"-mode", "write",
		"-field-layout", fieldLayoutWide10,
		"-points", "40",
		"-series", "4",
		"-write-batch-size", "7",
		"-memtable-max-samples", "40",
		"-compaction-enabled",
		"-compaction-level0-part-limit", "2",
		"-compaction-level0-size-limit", "1024",
		"-compaction-max-output-part-bytes", "2048",
		"-compaction-levels", "0:2:0:2048,1:2:0:4096",
		"-compaction-max-cascade-steps", "4",
		"-compaction-background-interval", "10ms",
		"-compression-algorithm", "lz4",
		"-flush-on-exit",
	})
	if err != nil {
		t.Fatalf("run(storage params) error = %v", err)
	}
}

func TestParseConfigStorageParameters(t *testing.T) {
	cfg, err := parseConfig([]string{
		"-write-batch-size", "2048",
		"-memtable-max-samples", "1000000",
		"-compaction-enabled",
		"-compaction-level0-part-limit", "8",
		"-compaction-level0-size-limit", "1048576",
		"-compaction-max-output-part-bytes", "268435456",
		"-compaction-levels", "0:8:1048576:268435456,1:4:0:536870912",
		"-compaction-max-cascade-steps", "6",
		"-compaction-background-interval", "250ms",
		"-compression-algorithm", "zstd",
		"-flush-on-exit",
	})
	if err != nil {
		t.Fatalf("parseConfig(storage params) error = %v", err)
	}
	if cfg.writeBatchSize != 2048 {
		t.Fatalf("writeBatchSize = %d, want 2048", cfg.writeBatchSize)
	}
	if cfg.memTableMaxSamples != 1000000 {
		t.Fatalf("memTableMaxSamples = %d, want 1000000", cfg.memTableMaxSamples)
	}
	if !cfg.compactionEnabled {
		t.Fatal("compactionEnabled = false, want true")
	}
	if cfg.compactionLevel0PartLimit != 8 {
		t.Fatalf("compactionLevel0PartLimit = %d, want 8", cfg.compactionLevel0PartLimit)
	}
	if cfg.compactionLevel0SizeLimit != 1048576 {
		t.Fatalf("compactionLevel0SizeLimit = %d, want 1048576", cfg.compactionLevel0SizeLimit)
	}
	if cfg.compactionMaxOutputPartBytes != 268435456 {
		t.Fatalf("compactionMaxOutputPartBytes = %d, want 268435456", cfg.compactionMaxOutputPartBytes)
	}
	if len(cfg.compactionLevels) != 2 || cfg.compactionLevels[1].Level != 1 {
		t.Fatalf("compactionLevels = %#v, want two parsed levels", cfg.compactionLevels)
	}
	if cfg.compactionLevels[1].MaxOutputPartBytes != 536870912 {
		t.Fatalf("level 1 max output = %d, want 536870912", cfg.compactionLevels[1].MaxOutputPartBytes)
	}
	if cfg.compactionMaxCascadeSteps != 6 {
		t.Fatalf("compactionMaxCascadeSteps = %d, want 6", cfg.compactionMaxCascadeSteps)
	}
	if cfg.compactionBackgroundInterval != 250*time.Millisecond {
		t.Fatalf("compactionBackgroundInterval = %s, want 250ms", cfg.compactionBackgroundInterval)
	}
	if cfg.compressionAlgorithm != "zstd" {
		t.Fatalf("compressionAlgorithm = %q, want zstd", cfg.compressionAlgorithm)
	}
	if !cfg.flushOnExit {
		t.Fatal("flushOnExit = false, want true")
	}
}

func TestStorageOptions(t *testing.T) {
	cfg := config{
		memTableMaxSamples:           1234,
		compactionEnabled:            true,
		compactionLevel0PartLimit:    3,
		compactionLevel0SizeLimit:    4096,
		compactionMaxOutputPartBytes: 8192,
		compactionLevels: []mts.CompactionLevelOptions{
			{Level: 0, PartLimit: 3, SizeLimit: 4096, MaxOutputPartBytes: 8192},
			{Level: 1, PartLimit: 2, MaxOutputPartBytes: 16384},
		},
		compactionMaxCascadeSteps:    5,
		compactionBackgroundInterval: time.Second,
		compressionAlgorithm:         "snappy",
	}
	opts := storageOptions("/tmp/mts-test", cfg)
	if opts.Path != "/tmp/mts-test" {
		t.Fatalf("Path = %q, want /tmp/mts-test", opts.Path)
	}
	if opts.MemTableMaxSamples != 1234 {
		t.Fatalf("MemTableMaxSamples = %d, want 1234", opts.MemTableMaxSamples)
	}
	if !opts.Compaction.Enabled {
		t.Fatal("Compaction.Enabled = false, want true")
	}
	if opts.Compaction.Level0PartLimit != 3 {
		t.Fatalf("Level0PartLimit = %d, want 3", opts.Compaction.Level0PartLimit)
	}
	if opts.Compaction.Level0SizeLimit != 4096 {
		t.Fatalf("Level0SizeLimit = %d, want 4096", opts.Compaction.Level0SizeLimit)
	}
	if opts.Compaction.MaxOutputPartBytes != 8192 {
		t.Fatalf("MaxOutputPartBytes = %d, want 8192", opts.Compaction.MaxOutputPartBytes)
	}
	if len(opts.Compaction.Levels) != 2 || opts.Compaction.Levels[1].Level != 1 {
		t.Fatalf("Levels = %#v, want two levels", opts.Compaction.Levels)
	}
	if opts.Compaction.MaxCascadeSteps != 5 {
		t.Fatalf("MaxCascadeSteps = %d, want 5", opts.Compaction.MaxCascadeSteps)
	}
	if opts.Compaction.BackgroundInterval != time.Second {
		t.Fatalf("BackgroundInterval = %s, want 1s", opts.Compaction.BackgroundInterval)
	}
	if !opts.Compression.Enabled || opts.Compression.Algorithm != "snappy" {
		t.Fatalf("Compression = %#v, want enabled snappy", opts.Compression)
	}
	if compressionOptions(compressionOff).Enabled {
		t.Fatal("compressionOptions(off).Enabled = true, want false")
	}
}

func TestCollectRunMetrics(t *testing.T) {
	emptyMetrics, err := collectRunMetrics("")
	if err != nil {
		t.Fatalf("collectRunMetrics(empty) error = %v", err)
	}
	if emptyMetrics.heapSysBytes == 0 {
		t.Fatal("collectRunMetrics(empty).heapSysBytes = 0, want runtime metric")
	}
	if err := logStageMetrics("test", ""); err != nil {
		t.Fatalf("logStageMetrics(empty) error = %v", err)
	}

	root := t.TempDir()
	partDir := filepath.Join(root, "data", "db", "rp", "shards", "0", "sst-000001")
	if err := os.MkdirAll(partDir, 0700); err != nil {
		t.Fatalf("MkdirAll(part) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(partDir, "values.bin"), []byte("abcd"), 0600); err != nil {
		t.Fatalf("WriteFile(values) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.bin"), []byte("xy"), 0600); err != nil {
		t.Fatalf("WriteFile(other) error = %v", err)
	}
	metrics, err := collectRunMetrics(root)
	if err != nil {
		t.Fatalf("collectRunMetrics() error = %v", err)
	}
	if metrics.sstableCount != 1 {
		t.Fatalf("sstableCount = %d, want 1", metrics.sstableCount)
	}
	if metrics.dataDirBytes != 6 {
		t.Fatalf("dataDirBytes = %d, want 6", metrics.dataDirBytes)
	}
	if metrics.heapSysBytes == 0 {
		t.Fatal("heapSysBytes = 0, want runtime metric")
	}
	if metrics.heapTotalAllocBytes == 0 {
		t.Fatal("heapTotalAllocBytes = 0, want runtime metric")
	}
	if metrics.mallocs == 0 {
		t.Fatal("mallocs = 0, want runtime metric")
	}
	if metrics.pauseTotalNs == 0 && metrics.numGC > 0 {
		t.Fatal("pauseTotalNs = 0 with numGC > 0, want GC pause metric")
	}
}

func TestParseProcStatusRSS(t *testing.T) {
	rss, peak, err := parseProcStatusRSS("Name:\tmts\nVmHWM:\t  456 kB\nVmRSS:\t  123 kB\n")
	if err != nil {
		t.Fatalf("parseProcStatusRSS() error = %v", err)
	}
	if rss != 123*1024 {
		t.Fatalf("rss = %d, want %d", rss, 123*1024)
	}
	if peak != 456*1024 {
		t.Fatalf("peak = %d, want %d", peak, 456*1024)
	}
	if _, _, err := parseProcStatusRSS("VmRSS:\tbad kB\n"); err == nil {
		t.Fatal("parseProcStatusRSS(bad) error = nil, want error")
	}
}

func TestWorkloadsUsePrebuiltPoints(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"write", "compact"} {
		t.Run(mode, func(t *testing.T) {
			eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			cfg := config{mode: mode, points: 32, series: 4, prebuildPoints: true}
			cfg.prebuilt = buildWorkloadPoints(cfg)
			if err := runWorkload(ctx, eng, cfg); err != nil {
				closeErr := eng.Close(ctx)
				t.Fatalf("runWorkload(%s) error = %v close = %v", mode, err, closeErr)
			}
			if err := eng.Close(ctx); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
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

func TestRunReadMode(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	err := run([]string{
		"-data-dir", dataDir,
		"-mode", "write",
		"-points", "200",
		"-series", "10",
		"-query-repeat", "2",
		"-compression-algorithm", "lz4",
		"-flush-on-exit",
	})
	if err != nil {
		t.Fatalf("run(write) error = %v", err)
	}
	err = run([]string{
		"-data-dir", dataDir,
		"-mode", "read",
		"-points", "200",
		"-series", "10",
		"-query-repeat", "2",
		"-compression-algorithm", "lz4",
	})
	if err != nil {
		t.Fatalf("run(read) error = %v", err)
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
		{name: "bad compression algorithm", args: []string{"-compression-algorithm", "gzip", "-points", "1", "-series", "1"}},
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
		{name: "zero write batch size", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 0, memTableMaxSamples: 1}},
		{name: "zero memtable samples", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 0}},
		{name: "negative level0 part limit", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 1, compactionLevel0PartLimit: -1}},
		{name: "negative level0 size limit", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 1, compactionLevel0SizeLimit: -1}},
		{name: "negative max output part bytes", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 1, compactionMaxOutputPartBytes: -1}},
		{name: "negative max cascade steps", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 1, compactionMaxCascadeSteps: -1}},
		{name: "negative background interval", cfg: config{mode: "query", points: 1, series: 1, writeBatchSize: 1, memTableMaxSamples: 1, compactionBackgroundInterval: -time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateConfig(tt.cfg); err == nil {
				t.Fatal("validateConfig() error = nil, want error")
			}
		})
	}
}

func TestParseCompactionLevelsRejectsInvalidSpec(t *testing.T) {
	tests := []string{
		"0:1:2",
		"x:1:2:3",
		"0:-1:2:3",
		"0:1:-2:3",
		"0:1:2:-3",
	}
	for _, spec := range tests {
		t.Run(spec, func(t *testing.T) {
			if _, err := parseCompactionLevels(spec); err == nil {
				t.Fatal("parseCompactionLevels() error = nil, want error")
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
