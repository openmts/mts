package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/storagefs"
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
		{"-memtable-max-samples", "0"},
		{"-query-limit", "0"},
		{"-shard-duration", "0"},
		{"-timestamp-step", "0"},
		{"-query-start", "10", "-query-end", "10"},
		{"-mode", "bad"},
		{"-profile", "bad"},
		{"-ingest-path", "bad"},
		{"-compression-algorithm", "gzip"},
		{"-durability", "bad"},
	} {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%v) error = nil, want error", args)
		}
	}
}

func TestParseConfigProfilesAndThresholds(t *testing.T) {
	cfg, err := parseConfig([]string{
		"-profile", "quick",
		"-max-rss-bytes", "1024",
		"-max-sstable-count", "3",
		"-max-compaction-backlog", "1",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.profile != "quick" || cfg.points != 10000 || cfg.maxRSSBytes != 1024 {
		t.Fatalf("config = %#v, want quick profile and rss threshold", cfg)
	}
	if cfg.mode != "compact" {
		t.Fatalf("mode = %q, want compact", cfg.mode)
	}
	if cfg.ingestPath != "typed" {
		t.Fatalf("ingestPath = %q, want typed", cfg.ingestPath)
	}
	if cfg.memTableMaxSamples != defaultMemTableMaxSamples {
		t.Fatalf("memTableMaxSamples = %d, want %d", cfg.memTableMaxSamples, defaultMemTableMaxSamples)
	}
	if cfg.compressionAlgorithm != compressionOff {
		t.Fatalf("compressionAlgorithm = %q, want %q", cfg.compressionAlgorithm, compressionOff)
	}
	if cfg.durability != durabilityBuffered {
		t.Fatalf("durability = %q, want %q", cfg.durability, durabilityBuffered)
	}
	if cfg.queryLimit != defaultQueryLimit {
		t.Fatalf("queryLimit = %d, want %d", cfg.queryLimit, defaultQueryLimit)
	}
	if !cfg.verify {
		t.Fatal("verify = false, want default true")
	}
	if cfg.maxSSTableCount != 3 || cfg.maxCompactionBacklog != 1 {
		t.Fatalf("config thresholds = %#v, want sstable/backlog thresholds", cfg)
	}
}

func TestParseConfigAcceptsVerifyFlag(t *testing.T) {
	cfg, err := parseConfig([]string{"-points", "10", "-verify=false"})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.verify {
		t.Fatal("verify = true, want false")
	}
}

func TestParseConfigAcceptsCompressionAlgorithms(t *testing.T) {
	for _, algorithm := range []string{"off", "none", "snappy", "lz4", "zstd"} {
		t.Run(algorithm, func(t *testing.T) {
			cfg, err := parseConfig([]string{"-points", "10", "-compression-algorithm", algorithm})
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if cfg.compressionAlgorithm != algorithm {
				t.Fatalf("compressionAlgorithm = %q, want %q", cfg.compressionAlgorithm, algorithm)
			}
		})
	}
}

func TestParseConfigAcceptsDurabilityModes(t *testing.T) {
	for _, mode := range []string{
		durabilityBuffered,
		durabilityWALSync,
		durabilityWriteSync,
		durabilityStrictFlush,
	} {
		t.Run(mode, func(t *testing.T) {
			cfg, err := parseConfig([]string{"-points", "10", "-durability", mode})
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if cfg.durability != mode {
				t.Fatalf("durability = %q, want %q", cfg.durability, mode)
			}
		})
	}
}

func TestDurabilityOptionsMapToEngineAndWriteOptions(t *testing.T) {
	tests := []struct {
		name      string
		wantWAL   bool
		wantWrite bool
		wantFlush bool
	}{
		{name: durabilityBuffered},
		{name: durabilityWALSync, wantWAL: true},
		{name: durabilityWriteSync, wantWrite: true},
		{name: durabilityStrictFlush, wantWAL: true, wantWrite: true, wantFlush: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durabilityOptions(tt.name)
			if got.walSync != tt.wantWAL || got.writeSync != tt.wantWrite || got.flushSync != tt.wantFlush {
				t.Fatalf("durabilityOptions(%q) = %#v", tt.name, got)
			}
		})
	}
}

func TestScaleCompressionOptions(t *testing.T) {
	if scaleCompressionOptions("off").Enabled {
		t.Fatal("scaleCompressionOptions(off).Enabled = true, want false")
	}
	got := scaleCompressionOptions("lz4")
	if !got.Enabled || got.Algorithm != "lz4" || got.MinPageValues != 1 {
		t.Fatalf("scaleCompressionOptions(lz4) = %#v, want enabled lz4", got)
	}
}

func TestParseConfigAcceptsMemTableMaxSamples(t *testing.T) {
	cfg, err := parseConfig([]string{
		"-points", "10",
		"-memtable-max-samples", "128",
		"-shard-duration", "2h",
		"-timestamp-step", "2s",
		"-query-start", "4",
		"-query-end", "8",
		"-query-limit", "3",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.memTableMaxSamples != 128 {
		t.Fatalf("memTableMaxSamples = %d, want 128", cfg.memTableMaxSamples)
	}
	if cfg.queryStart != 4 || cfg.queryEnd != 8 || cfg.queryLimit != 3 {
		t.Fatalf("query config = start %d end %d limit %d, want 4/8/3",
			cfg.queryStart, cfg.queryEnd, cfg.queryLimit)
	}
	if cfg.shardDuration != 2*time.Hour || cfg.timestampStep != 2*time.Second {
		t.Fatalf("time config = shard %s step %s, want 2h/2s", cfg.shardDuration, cfg.timestampStep)
	}
}

func TestScaleQuerySpecUsesMiddleLimitedWindow(t *testing.T) {
	spec := scaleQuerySpec(config{points: 1_000_000, queryLimit: 2_000})
	if spec.startIndex != 499_000 || spec.endIndex != 500_999 ||
		spec.start != int64(499_000*time.Second) ||
		spec.end != int64(500_999*time.Second) ||
		spec.limit != 2_000 {
		t.Fatalf("scaleQuerySpec(1M) = %#v, want middle 2000 rows", spec)
	}
	small := scaleQuerySpec(config{points: 10, queryLimit: 2_000})
	if small.start != 0 || small.end != int64(9*time.Second) || small.limit != 2_000 {
		t.Fatalf("scaleQuerySpec(small) = %#v, want full small range with limit 2000", small)
	}
	override := scaleQuerySpec(config{points: 100, queryStart: int64(30 * time.Second), queryEnd: int64(40 * time.Second), queryLimit: 5})
	if override.startIndex != 30 || override.endIndex != 40 || override.limit != 5 {
		t.Fatalf("scaleQuerySpec(override) = %#v, want explicit range and limit", override)
	}
}

func TestExpectedQueryRowsHonorsInclusiveRangeAndLimit(t *testing.T) {
	tests := []struct {
		name  string
		query querySpec
		want  int
	}{
		{
			name:  "inclusive range under limit",
			query: querySpec{startIndex: 7, endIndex: 12, limit: 2_000},
			want:  6,
		},
		{
			name:  "limit smaller than range",
			query: querySpec{startIndex: 10, endIndex: 20, limit: 5},
			want:  5,
		},
		{
			name:  "empty range",
			query: querySpec{startIndex: 20, endIndex: 10, limit: 5},
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectedQueryRows(tt.query); got != tt.want {
				t.Fatalf("expectedQueryRows() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRunFailsWhenThresholdExceeded(t *testing.T) {
	cfg := config{
		maxRSSBytes:          1024,
		maxSSTableCount:      1,
		maxCompactionBacklog: 0,
	}
	out := report{
		RSSPeakBytes:    2048,
		SSTableCount:    2,
		CompactionStats: structCompactionStats(1),
	}
	if err := enforceThresholds(cfg, out); err == nil {
		t.Fatal("enforceThresholds() error = nil, want threshold error")
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
	for _, mode := range []string{"write", "query", "compact", "write-query-compact", "restart"} {
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

func TestParseConfigAcceptsWriteQueryCompactMode(t *testing.T) {
	cfg, err := parseConfig([]string{"-points", "10", "-mode", "write-query-compact"})
	if err != nil {
		t.Fatalf("parseConfig(write-query-compact) error = %v", err)
	}
	if cfg.mode != "write-query-compact" {
		t.Fatalf("mode = %q, want write-query-compact", cfg.mode)
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
		"compression_algorithm",
		"durability",
		"wal_sync",
		"write_sync",
		"flush_sync",
		"query_start_time",
		"query_end_time",
		"query_limit",
		"verify",
		"shard_duration_nanos",
		"timestamp_step_nanos",
		"shard_count",
		"shard_sstable_distribution",
		"level_distribution",
		"read_amplification",
		"write_amplification",
		"space_amplification",
		"query_latency_nanos",
		"cold_query_latency_nanos",
		"hot_query_latency_nanos",
		"backlog_drain_nanos",
		"write_duration_nanos",
		"write_throughput",
		"compaction_duration_nanos",
		"sstable_count_before_compaction",
		"sstable_count_after_compaction",
		"level_distribution_before_compaction",
		"level_distribution_after_compaction",
		"compaction_result",
		"compaction_stats",
		"errors",
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("report json %s missing %s", text, key)
		}
	}
}

func TestCompactModeReportsWriteAndCompactionStats(t *testing.T) {
	result, err := runWorkloadDetailed(t.TempDir(), config{
		mode:               "compact",
		ingestPath:         "typed",
		points:             512,
		batchSize:          32,
		memTableMaxSamples: 64,
	})
	if err != nil {
		t.Fatalf("runWorkloadDetailed(compact) error = %v", err)
	}
	if result.writeDuration <= 0 {
		t.Fatalf("writeDuration = %s, want positive", result.writeDuration)
	}
	if result.compactionDuration <= 0 {
		t.Fatalf("compactionDuration = %s, want positive", result.compactionDuration)
	}
	if result.sstableCountBeforeCompaction <= 1 {
		t.Fatalf("sstableCountBeforeCompaction = %d, want more than one", result.sstableCountBeforeCompaction)
	}
	if result.sstableCountAfterCompaction <= 0 {
		t.Fatalf("sstableCountAfterCompaction = %d, want positive", result.sstableCountAfterCompaction)
	}
	if result.sstableCountAfterCompaction > result.sstableCountBeforeCompaction {
		t.Fatalf(
			"sstable count after compaction = %d, want <= before %d",
			result.sstableCountAfterCompaction,
			result.sstableCountBeforeCompaction,
		)
	}
	if len(result.levelDistributionBeforeCompaction) == 0 {
		t.Fatal("levelDistributionBeforeCompaction is empty")
	}
	if len(result.levelDistributionAfterCompaction) == 0 {
		t.Fatal("levelDistributionAfterCompaction is empty")
	}
	if result.compactionResult.InputParts == 0 || result.compactionResult.OutputParts == 0 {
		t.Fatalf("compactionResult = %#v, want input and output parts", result.compactionResult)
	}
}

func TestCompactModeDistributesGeneratedDataAcrossShards(t *testing.T) {
	result, err := runWorkloadDetailed(t.TempDir(), config{
		mode:               "compact",
		ingestPath:         "typed",
		points:             12,
		batchSize:          3,
		memTableMaxSamples: 6,
		shardDuration:      3 * time.Second,
		timestampStep:      time.Second,
		queryLimit:         4,
	})
	if err != nil {
		t.Fatalf("runWorkloadDetailed(cross shard) error = %v", err)
	}
	if result.rows != 4 {
		t.Fatalf("rows = %d, want 4", result.rows)
	}
	if result.sstableCountAfterCompaction < 4 {
		t.Fatalf("sstableCountAfterCompaction = %d, want at least one per shard", result.sstableCountAfterCompaction)
	}
}

func TestTimedQueryLimitedRowsCountsAndValidatesMiddleRows(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.WriteTypedBatch(ctx, scaleTypedBatch(0, 20, scaleHostCache(100)), mts.WriteOptions{}); err != nil {
		t.Fatalf("WriteTypedBatch() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	spec := scaleQuerySpec(config{points: 20, queryLimit: 6})
	rows, latency, err := timedQueryLimitedRows(ctx, eng, spec, true)
	if err != nil {
		t.Fatalf("timedQueryLimitedRows() error = %v", err)
	}
	if rows != 6 {
		t.Fatalf("timedQueryLimitedRows() rows = %d, want 6", rows)
	}
	if latency <= 0 {
		t.Fatalf("timedQueryLimitedRows() latency = %s, want positive", latency)
	}
}

func structCompactionStats(backlog int) mts.CompactionStats {
	return mts.CompactionStats{Backlog: backlog}
}

func TestScaleTypedBatch(t *testing.T) {
	batch := newScaleTypedBatchBuilder(3).Build(2, 5, scaleHostCache(100), config{timestampStep: 2 * time.Second})
	if len(batch.Timestamps) != 3 {
		t.Fatalf("typed timestamps len = %d, want 3", len(batch.Timestamps))
	}
	if batch.Timestamps[0] != int64(4*time.Second) {
		t.Fatalf("typed first timestamp = %d, want 4s", batch.Timestamps[0])
	}
	if len(batch.Tags) != 1 || batch.Tags[0].Values[0] != "host-002" {
		t.Fatalf("typed tags = %#v, want first host-002", batch.Tags)
	}
	if len(batch.Fields) != 10 {
		t.Fatalf("typed fields len = %d, want 10", len(batch.Fields))
	}
	if batch.Fields[0].Float64Values[2] != 4 {
		t.Fatalf("f0 last value = %v, want 4", batch.Fields[0].Float64Values[2])
	}
	if !batch.Fields[9].BoolValues[0] {
		t.Fatalf("b0 first value = false, want true")
	}
}

func TestScaleTypedBatchBuilderReusesBackingArrays(t *testing.T) {
	builder := newScaleTypedBatchBuilder(4)
	hosts := scaleHostCache(100)
	first := builder.Build(0, 4, hosts, config{})
	second := builder.Build(4, 8, hosts, config{})
	if len(first.Timestamps) == 0 || len(second.Timestamps) == 0 {
		t.Fatal("typed batches are empty")
	}
	if &first.Timestamps[0] != &second.Timestamps[0] {
		t.Fatal("timestamp backing array was not reused")
	}
	if &first.Tags[0].Values[0] != &second.Tags[0].Values[0] {
		t.Fatal("tag backing array was not reused")
	}
	if &first.Fields[0].Float64Values[0] != &second.Fields[0].Float64Values[0] {
		t.Fatal("field backing array was not reused")
	}
	if second.Timestamps[0] != int64(4*time.Second) || second.Fields[0].Float64Values[0] != 4 {
		t.Fatalf("second batch first row = time %d f0 %v, want index 4 at 4s",
			second.Timestamps[0], second.Fields[0].Float64Values[0])
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

func TestRunWritesJSONReportFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "reports", "storage.json")
	if err := run([]string{"-points", "8", "-batch-size", "4", "-out", out}); err != nil {
		t.Fatalf("run(out) error = %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile(out) error = %v", err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(out) error = %v data=%s", err, string(data))
	}
	if got.Points != 8 || !got.Verify {
		t.Fatalf("report = %#v, want points and verify", got)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("Stat(out) error = %v", err)
	}
	// Windows 使用 ACL 进行访问控制，Unix 权限位无实际意义
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("report mode = %04o, want 0600", info.Mode().Perm())
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
