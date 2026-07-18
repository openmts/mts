package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/sstable"
)

const defaultMemTableMaxSamples = 8192

const defaultQueryLimit = 2000

const defaultScaleShardDuration = 24 * time.Hour

const defaultScaleTimestampStep = time.Second

const (
	compressionOff  = "off"
	compressionNone = "none"
)

const (
	durabilityBuffered    = "buffered"
	durabilityWALSync     = "wal-sync"
	durabilityWriteSync   = "write-sync"
	durabilityStrictFlush = "strict-flush"
)

const (
	modeWrite             = "write"
	modeQuery             = "query"
	modeCompact           = "compact"
	modeWriteQueryCompact = "write-query-compact"
	modeRestart           = "restart"
)

type report struct {
	Profile                           string               `json:"profile"`
	Mode                              string               `json:"mode"`
	IngestPath                        string               `json:"ingest_path"`
	CompressionAlgorithm              string               `json:"compression_algorithm"`
	Durability                        string               `json:"durability"`
	WALSync                           bool                 `json:"wal_sync"`
	WriteSync                         bool                 `json:"write_sync"`
	FlushSync                         bool                 `json:"flush_sync"`
	Points                            int                  `json:"points"`
	QueryStartTime                    int64                `json:"query_start_time"`
	QueryEndTime                      int64                `json:"query_end_time"`
	QueryLimit                        int                  `json:"query_limit"`
	Verify                            bool                 `json:"verify"`
	ShardDurationNanos                int64                `json:"shard_duration_nanos"`
	TimestampStepNanos                int64                `json:"timestamp_step_nanos"`
	Duration                          time.Duration        `json:"duration"`
	Throughput                        float64              `json:"throughput"`
	WriteDurationNanos                int64                `json:"write_duration_nanos"`
	WriteThroughput                   float64              `json:"write_throughput"`
	CompactionDurationNanos           int64                `json:"compaction_duration_nanos"`
	HeapAlloc                         uint64               `json:"heap_alloc"`
	HeapSys                           uint64               `json:"heap_sys"`
	TotalAlloc                        uint64               `json:"total_alloc"`
	Mallocs                           uint64               `json:"mallocs"`
	Frees                             uint64               `json:"frees"`
	NumGC                             uint32               `json:"num_gc"`
	RSSPeakBytes                      int64                `json:"rss_peak_bytes"`
	Rows                              int                  `json:"rows"`
	DataBytes                         int64                `json:"data_bytes"`
	ShardCount                        int                  `json:"shard_count"`
	ShardSSTableDistribution          map[string]int       `json:"shard_sstable_distribution"`
	SSTableCount                      int                  `json:"sstable_count"`
	SSTableCountBeforeCompaction      int                  `json:"sstable_count_before_compaction"`
	SSTableCountAfterCompaction       int                  `json:"sstable_count_after_compaction"`
	LevelDistribution                 map[int]int          `json:"level_distribution"`
	LevelDistributionBeforeCompaction map[int]int          `json:"level_distribution_before_compaction"`
	LevelDistributionAfterCompaction  map[int]int          `json:"level_distribution_after_compaction"`
	ReadAmplification                 float64              `json:"read_amplification"`
	WriteAmplification                float64              `json:"write_amplification"`
	SpaceAmplification                float64              `json:"space_amplification"`
	QueryLatencyNanos                 int64                `json:"query_latency_nanos"`
	ColdQueryLatency                  int64                `json:"cold_query_latency_nanos"`
	HotQueryLatency                   int64                `json:"hot_query_latency_nanos"`
	BacklogDrainNanos                 int64                `json:"backlog_drain_nanos"`
	CompactionResult                  mts.CompactionResult `json:"compaction_result"`
	CompactionStats                   mts.CompactionStats  `json:"compaction_stats"`
	Errors                            []string             `json:"errors"`
}

type config struct {
	profile              string
	mode                 string
	ingestPath           string
	points               int
	batchSize            int
	memTableMaxSamples   int
	compressionAlgorithm string
	valuePageSamples     int
	durability           string
	queryStart           int64
	queryEnd             int64
	queryLimit           int
	verify               bool
	shardDuration        time.Duration
	timestampStep        time.Duration
	dataDir              string
	outPath              string
	baseline             string
	maxRegressionPercent float64
	maxRSSBytes          int64
	maxSSTableCount      int
	maxCompactionBacklog int
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (err error) {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	dir, cleanup, err := prepareDir(cfg.dataDir)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()
	started := time.Now()
	workload, err := runWorkloadDetailed(dir, cfg)
	if err != nil {
		return err
	}
	duration := time.Since(started)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	dataBytes, err := dirSize(dir)
	if err != nil {
		return err
	}
	shardCount, err := countShards(dir)
	if err != nil {
		return err
	}
	shardSSTables, err := shardSSTableDistribution(dir)
	if err != nil {
		return err
	}
	tableCount, err := countSSTables(dir)
	if err != nil {
		return err
	}
	levelDistribution, err := levelDistribution(dir)
	if err != nil {
		return err
	}
	logicalBytes := logicalInputBytes(cfg.points)
	query := scaleQuerySpec(cfg)
	durability := durabilityOptions(cfg.durability)
	out := report{
		Profile:                           cfg.profile,
		Mode:                              cfg.mode,
		IngestPath:                        cfg.ingestPath,
		CompressionAlgorithm:              cfg.compressionAlgorithm,
		Durability:                        cfg.durability,
		WALSync:                           durability.walSync,
		WriteSync:                         durability.writeSync,
		FlushSync:                         durability.flushSync,
		Points:                            cfg.points,
		QueryStartTime:                    query.start,
		QueryEndTime:                      query.end,
		QueryLimit:                        query.limit,
		Verify:                            cfg.verify,
		ShardDurationNanos:                shardDuration(cfg).Nanoseconds(),
		TimestampStepNanos:                timestampStepNanos(cfg),
		Duration:                          duration,
		Throughput:                        throughput(cfg.points, duration),
		WriteDurationNanos:                workload.writeDuration.Nanoseconds(),
		WriteThroughput:                   throughput(cfg.points, workload.writeDuration),
		CompactionDurationNanos:           workload.compactionDuration.Nanoseconds(),
		HeapAlloc:                         mem.HeapAlloc,
		HeapSys:                           mem.HeapSys,
		TotalAlloc:                        mem.TotalAlloc,
		Mallocs:                           mem.Mallocs,
		Frees:                             mem.Frees,
		NumGC:                             mem.NumGC,
		RSSPeakBytes:                      rssPeakBytes(),
		Rows:                              workload.rows,
		DataBytes:                         dataBytes,
		ShardCount:                        shardCount,
		ShardSSTableDistribution:          shardSSTables,
		SSTableCount:                      tableCount,
		SSTableCountBeforeCompaction:      workload.sstableCountBeforeCompaction,
		SSTableCountAfterCompaction:       workload.sstableCountAfterCompaction,
		LevelDistribution:                 levelDistribution,
		LevelDistributionBeforeCompaction: workload.levelDistributionBeforeCompaction,
		LevelDistributionAfterCompaction:  workload.levelDistributionAfterCompaction,
		ReadAmplification:                 readAmplification(tableCount, workload.rows),
		WriteAmplification:                amplificationRatio(dataBytes, logicalBytes),
		SpaceAmplification:                amplificationRatio(dataBytes, logicalBytes),
		QueryLatencyNanos:                 workload.queryLatency.Nanoseconds(),
		ColdQueryLatency:                  workload.coldQueryLatency.Nanoseconds(),
		HotQueryLatency:                   workload.hotQueryLatency.Nanoseconds(),
		BacklogDrainNanos:                 workload.backlogDrain.Nanoseconds(),
		CompactionResult:                  workload.compactionResult,
		CompactionStats:                   workload.compactionStats,
		Errors:                            []string{},
	}
	if err := compareBaseline(cfg, out); err != nil {
		return err
	}
	if err := enforceThresholds(cfg, out); err != nil {
		return err
	}
	if err := writeReportFile(cfg.outPath, out); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func parseConfig(args []string) (config, error) {
	flags := flag.NewFlagSet("storage_10m", flag.ContinueOnError)
	profile := flags.String("profile", "quick", "profile: quick|standard|soak")
	mode := flags.String("mode", "compact", "mode")
	ingestPath := flags.String("ingest-path", "typed", "ingest path: typed|public")
	points := flags.Int("points", 0, "points")
	batchSize := flags.Int("batch-size", 1024, "batch size")
	memTableMaxSamples := flags.Int(
		"memtable-max-samples",
		defaultMemTableMaxSamples,
		"trigger MemTable flush after this many samples",
	)
	compressionAlgorithm := flags.String(
		"compression-algorithm",
		compressionOff,
		"payload compression algorithm: off|none|snappy|lz4|zstd",
	)
	valuePageSamples := flags.Int(
		"value-page-samples",
		0,
		"SSTable value page samples; 0 uses engine default",
	)
	durability := flags.String(
		"durability",
		durabilityBuffered,
		"write durability mode: buffered|wal-sync|write-sync|strict-flush",
	)
	queryStart := flags.Int64("query-start", -1, "query start timestamp; default centers query window")
	queryEnd := flags.Int64("query-end", -1, "query end timestamp; default centers query window")
	queryLimit := flags.Int("query-limit", defaultQueryLimit, "row-level query limit")
	verify := flags.Bool("verify", true, "validate query result row count and generated row values")
	shardDuration := flags.Duration("shard-duration", defaultScaleShardDuration, "storage shard duration")
	timestampStep := flags.Duration("timestamp-step", defaultScaleTimestampStep, "logical timestamp interval between generated rows")
	dataDir := flags.String("data-dir", "", "data directory")
	outPath := flags.String("out", "", "write final json report to file")
	baseline := flags.String("baseline", "", "baseline report json")
	maxRegression := flags.Float64("max-regression-percent", 20, "max allowed regression percent")
	maxRSSBytes := flags.Int64("max-rss-bytes", 0, "max allowed RSS peak bytes")
	maxSSTables := flags.Int("max-sstable-count", 0, "max allowed SSTable count")
	maxBacklog := flags.Int("max-compaction-backlog", 0, "max allowed compaction backlog")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	visited := visitedFlags(flags)
	if _, ok := visited["points"]; !ok {
		*points = profilePoints(*profile)
	}
	if *points <= 0 || *batchSize <= 0 || *memTableMaxSamples <= 0 ||
		*queryLimit <= 0 || *shardDuration <= 0 || *timestampStep <= 0 {
		return config{}, fmt.Errorf("points, batch-size, memtable-max-samples, query-limit, shard-duration and timestamp-step must be positive")
	}
	if err := validateQueryRange(*queryStart, *queryEnd); err != nil {
		return config{}, err
	}
	if !validProfile(*profile) {
		return config{}, fmt.Errorf("unsupported profile %q", *profile)
	}
	if !validIngestPath(*ingestPath) {
		return config{}, fmt.Errorf("unsupported ingest path %q", *ingestPath)
	}
	if !validCompressionAlgorithm(*compressionAlgorithm) {
		return config{}, fmt.Errorf("unsupported compression algorithm %q", *compressionAlgorithm)
	}
	if !validDurability(*durability) {
		return config{}, fmt.Errorf("unsupported durability %q", *durability)
	}
	if *maxRSSBytes < 0 || *maxSSTables < 0 || *maxBacklog < 0 {
		return config{}, fmt.Errorf("thresholds must be non-negative")
	}
	switch *mode {
	case modeWrite, modeQuery, modeCompact, modeWriteQueryCompact, modeRestart:
	default:
		return config{}, fmt.Errorf("unsupported mode %q", *mode)
	}
	return config{
		profile:              *profile,
		mode:                 *mode,
		ingestPath:           *ingestPath,
		points:               *points,
		batchSize:            *batchSize,
		memTableMaxSamples:   *memTableMaxSamples,
		compressionAlgorithm: *compressionAlgorithm,
		valuePageSamples:     *valuePageSamples,
		durability:           *durability,
		queryStart:           *queryStart,
		queryEnd:             *queryEnd,
		queryLimit:           *queryLimit,
		verify:               *verify,
		shardDuration:        *shardDuration,
		timestampStep:        *timestampStep,
		dataDir:              *dataDir,
		outPath:              *outPath,
		baseline:             *baseline,
		maxRegressionPercent: *maxRegression,
		maxRSSBytes:          *maxRSSBytes,
		maxSSTableCount:      *maxSSTables,
		maxCompactionBacklog: *maxBacklog,
	}, nil
}

func writeReportFile(path string, out report) error {
	if path == "" {
		return nil
	}
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create report dir: %w", err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("chmod report dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode report file: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(clean, data, 0600); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}
	if err := os.Chmod(clean, 0600); err != nil {
		return fmt.Errorf("chmod report file: %w", err)
	}
	return nil
}

func visitedFlags(flags *flag.FlagSet) map[string]struct{} {
	visited := make(map[string]struct{})
	flags.Visit(func(item *flag.Flag) {
		visited[item.Name] = struct{}{}
	})
	return visited
}

func validProfile(profile string) bool {
	return profile == "quick" || profile == "standard" || profile == "soak"
}

func validIngestPath(path string) bool {
	return path == "typed" || path == "public"
}

func validCompressionAlgorithm(algorithm string) bool {
	switch algorithm {
	case compressionOff, compressionNone, "snappy", "lz4", "zstd":
		return true
	default:
		return false
	}
}

func validDurability(durability string) bool {
	switch durability {
	case durabilityBuffered, durabilityWALSync, durabilityWriteSync, durabilityStrictFlush:
		return true
	default:
		return false
	}
}

type durabilitySetting struct {
	walSync   bool
	writeSync bool
	flushSync bool
}

func durabilityOptions(durability string) durabilitySetting {
	switch durability {
	case durabilityWALSync:
		return durabilitySetting{walSync: true}
	case durabilityWriteSync:
		return durabilitySetting{writeSync: true}
	case durabilityStrictFlush:
		return durabilitySetting{walSync: true, writeSync: true, flushSync: true}
	default:
		return durabilitySetting{}
	}
}

func validateQueryRange(start int64, end int64) error {
	if start < -1 || end < -1 {
		return fmt.Errorf("query-start and query-end must be -1 or non-negative")
	}
	if start >= 0 && end >= 0 && start >= end {
		return fmt.Errorf("query-start must be less than query-end")
	}
	return nil
}

func profilePoints(profile string) int {
	switch profile {
	case "standard":
		return 1_000_000
	case "soak":
		return 10_000_000
	default:
		return 10_000
	}
}

func prepareDir(path string) (string, func() error, error) {
	if path != "" {
		if err := os.MkdirAll(path, 0700); err != nil {
			return "", nil, fmt.Errorf("create data dir: %w", err)
		}
		return path, func() error { return nil }, nil
	}
	dir, err := os.MkdirTemp("", "mts-scale-10m-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("chmod temp dir: %w", err)
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func runWorkload(dir string, cfg config) (int, error) {
	result, err := runWorkloadDetailed(dir, cfg)
	return result.rows, err
}

type workloadResult struct {
	rows                              int
	queryLatency                      time.Duration
	coldQueryLatency                  time.Duration
	hotQueryLatency                   time.Duration
	writeDuration                     time.Duration
	compactionDuration                time.Duration
	backlogDrain                      time.Duration
	sstableCountBeforeCompaction      int
	sstableCountAfterCompaction       int
	levelDistributionBeforeCompaction map[int]int
	levelDistributionAfterCompaction  map[int]int
	compactionResult                  mts.CompactionResult
	compactionStats                   mts.CompactionStats
}

type sstableSnapshot struct {
	count             int
	levelDistribution map[int]int
}

type querySpec struct {
	start         int64
	end           int64
	limit         int
	startIndex    int
	endIndex      int
	timestampStep int64
}

func runWorkloadDetailed(dir string, cfg config) (workloadResult, error) {
	ctx := context.Background()
	result, err := runOpenWorkload(ctx, dir, cfg)
	if err != nil || cfg.mode != modeRestart {
		return result, err
	}
	return queryReopenedWorkload(ctx, dir, cfg, result)
}

func runOpenWorkload(ctx context.Context, dir string, cfg config) (result workloadResult, err error) {
	eng, err := openScaleEngine(ctx, dir, cfg)
	if err != nil {
		return workloadResult{}, fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	if err := writeAndMaybeCompact(ctx, dir, eng, cfg, &result); err != nil {
		return workloadResult{}, err
	}
	if shouldQueryRows(cfg.mode) {
		if err := queryOpenEngine(ctx, eng, cfg, &result); err != nil {
			return workloadResult{}, err
		}
	}
	result.compactionStats = eng.CompactionStatsSnapshot()
	return result, nil
}

func writeAndMaybeCompact(
	ctx context.Context,
	dir string,
	eng *mts.Engine,
	cfg config,
	result *workloadResult,
) error {
	writeDuration, err := writeAndFlushScale(ctx, eng, cfg)
	if err != nil {
		return err
	}
	result.writeDuration = writeDuration
	beforeCompaction, err := captureSSTableSnapshot(dir)
	if err != nil {
		return fmt.Errorf("snapshot before compaction: %w", err)
	}
	result.setCompactionSnapshots(beforeCompaction, beforeCompaction)
	if shouldCompact(cfg.mode) {
		return compactScaleWorkload(ctx, dir, eng, beforeCompaction, result)
	}
	return nil
}

func compactScaleWorkload(
	ctx context.Context,
	dir string,
	eng *mts.Engine,
	beforeCompaction sstableSnapshot,
	result *workloadResult,
) error {
	compactStarted := time.Now()
	compactionResult, err := eng.CompactWithResult(ctx)
	if err != nil {
		return fmt.Errorf("compact: %w", err)
	}
	result.compactionDuration = time.Since(compactStarted)
	result.backlogDrain = result.compactionDuration
	result.compactionResult = compactionResult
	afterCompaction, err := captureSSTableSnapshot(dir)
	if err != nil {
		return fmt.Errorf("snapshot after compaction: %w", err)
	}
	result.setCompactionSnapshots(beforeCompaction, afterCompaction)
	return nil
}

func shouldQueryRows(mode string) bool {
	return mode == modeQuery || mode == modeCompact ||
		mode == modeWriteQueryCompact || mode == modeRestart
}

func shouldCompact(mode string) bool {
	return mode == modeCompact || mode == modeWriteQueryCompact
}

func queryOpenEngine(ctx context.Context, eng *mts.Engine, cfg config, result *workloadResult) error {
	query := scaleQuerySpec(cfg)
	rows, latency, err := timedQueryLimitedRows(ctx, eng, query, cfg.verify)
	if err != nil {
		return fmt.Errorf("query rows: %w", err)
	}
	_, hotLatency, err := timedQueryLimitedRows(ctx, eng, query, cfg.verify)
	if err != nil {
		return fmt.Errorf("query rows hot: %w", err)
	}
	result.rows = rows
	result.queryLatency = latency
	result.coldQueryLatency = latency
	result.hotQueryLatency = hotLatency
	return nil
}

func queryReopenedWorkload(
	ctx context.Context,
	dir string,
	cfg config,
	result workloadResult,
) (out workloadResult, err error) {
	out = result
	reopened, err := openScaleEngine(ctx, dir, cfg)
	if err != nil {
		return workloadResult{}, fmt.Errorf("reopen engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, reopened.Close(ctx))
	}()
	rows, latency, err := timedQueryLimitedRows(ctx, reopened, scaleQuerySpec(cfg), cfg.verify)
	if err != nil {
		return workloadResult{}, fmt.Errorf("query reopened: %w", err)
	}
	out.rows = rows
	out.queryLatency = latency
	return out, nil
}

func (r *workloadResult) setCompactionSnapshots(before sstableSnapshot, after sstableSnapshot) {
	r.sstableCountBeforeCompaction = before.count
	r.sstableCountAfterCompaction = after.count
	r.levelDistributionBeforeCompaction = before.levelDistribution
	r.levelDistributionAfterCompaction = after.levelDistribution
}

func openScaleEngine(ctx context.Context, dir string, cfg config) (*mts.Engine, error) {
	memTableMaxSamples := cfg.memTableMaxSamples
	if memTableMaxSamples <= 0 {
		memTableMaxSamples = defaultMemTableMaxSamples
	}
	durability := durabilityOptions(cfg.durability)
	return mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      shardDuration(cfg),
		MemTableMaxSamples: memTableMaxSamples,
		WAL:                mts.WALOptions{Sync: durability.walSync},
		FlushSync:          durability.flushSync,
		Compression:        scaleCompressionOptions(cfg.compressionAlgorithm, cfg.valuePageSamples),
	})
}

func scaleCompressionOptions(algorithm string, valuePageSamples int) mts.CompressionOptions {
	if algorithm == "" || algorithm == compressionOff {
		return mts.CompressionOptions{ValuePageSamples: valuePageSamples}
	}
	return mts.CompressionOptions{
		Enabled:          true,
		Algorithm:        algorithm,
		MinPageValues:    1,
		ValuePageSamples: valuePageSamples,
	}
}

func writeAndFlushScale(ctx context.Context, eng *mts.Engine, cfg config) (time.Duration, error) {
	started := time.Now()
	if err := writeScaleBatches(ctx, eng, cfg); err != nil {
		return time.Since(started), fmt.Errorf("write batches: %w", err)
	}
	if err := eng.Flush(ctx); err != nil {
		return time.Since(started), fmt.Errorf("flush: %w", err)
	}
	return time.Since(started), nil
}

func shardDuration(cfg config) time.Duration {
	if cfg.shardDuration <= 0 {
		return defaultScaleShardDuration
	}
	return cfg.shardDuration
}

func timestampStepNanos(cfg config) int64 {
	if cfg.timestampStep <= 0 {
		return int64(defaultScaleTimestampStep)
	}
	return int64(cfg.timestampStep)
}

func timestampForIndex(index int, step int64) int64 {
	return int64(index) * step
}

func logicalIndexForTimestamp(timestamp int64, step int64) (int, error) {
	if step <= 0 {
		return 0, fmt.Errorf("timestamp step must be positive")
	}
	if timestamp%step != 0 {
		return 0, fmt.Errorf("timestamp %d is not aligned to step %d", timestamp, step)
	}
	index := timestamp / step
	if index > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("timestamp index %d overflows int", index)
	}
	return int(index), nil
}

func ceilDivInt64(value int64, divisor int64) int {
	if value <= 0 {
		return 0
	}
	return int((value + divisor - 1) / divisor)
}

func floorDivInt64(value int64, divisor int64) int {
	if value < 0 {
		return -1
	}
	return int(value / divisor)
}

func scaleQuerySpec(cfg config) querySpec {
	limit := cfg.queryLimit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	points := cfg.points
	step := timestampStepNanos(cfg)
	start := cfg.queryStart
	end := cfg.queryEnd
	if start == 0 && end == 0 {
		start = -1
		end = -1
	}
	window := limit
	if window > points {
		window = points
	}
	startIndex := 0
	endIndex := points - 1
	if start < 0 && end < 0 {
		startIndex = (points - window) / 2
		endIndex = startIndex + window - 1
		start = timestampForIndex(startIndex, step)
		end = timestampForIndex(endIndex, step)
		return querySpec{
			start:         start,
			end:           end,
			limit:         limit,
			startIndex:    startIndex,
			endIndex:      endIndex,
			timestampStep: step,
		}
	}
	if start >= 0 {
		startIndex = ceilDivInt64(start, step)
	}
	if end >= 0 {
		endIndex = floorDivInt64(end, step)
	}
	if start >= 0 && end < 0 {
		endIndex = startIndex + window - 1
		end = timestampForIndex(endIndex, step)
	}
	if start < 0 && end >= 0 {
		startIndex = endIndex - window + 1
		start = timestampForIndex(startIndex, step)
	}
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex >= points {
		endIndex = points - 1
	}
	if startIndex > endIndex {
		startIndex = endIndex
	}
	if start < 0 {
		start = timestampForIndex(startIndex, step)
	}
	if end < 0 || end >= timestampForIndex(points, step) {
		end = timestampForIndex(endIndex, step)
	}
	return querySpec{
		start:         start,
		end:           end,
		limit:         limit,
		startIndex:    startIndex,
		endIndex:      endIndex,
		timestampStep: step,
	}
}

func timedQueryLimitedRows(
	ctx context.Context,
	eng *mts.Engine,
	query querySpec,
	verify bool,
) (int, time.Duration, error) {
	started := time.Now()
	iter, err := eng.QueryRowIterator(ctx, mts.Query{
		Measurement: "scale",
		StartTime:   query.start,
		EndTime:     query.end,
		Limit:       query.limit,
	})
	if err != nil {
		return 0, time.Since(started), err
	}
	rows := 0
	for iter.Next() {
		if verify {
			if err := validateScaleRow(iter.Row(), query); err != nil {
				return rows, time.Since(started), errors.Join(err, iter.Close())
			}
		}
		if !verify {
			_ = iter.Row()
		}
		rows++
	}
	if err := errors.Join(iter.Err(), iter.Close()); err != nil {
		return rows, time.Since(started), err
	}
	if verify {
		if want := expectedQueryRows(query); rows != want {
			return rows, time.Since(started), fmt.Errorf("query rows = %d, want %d", rows, want)
		}
	}
	return rows, time.Since(started), nil
}

func validateScaleRow(row mts.Row, query querySpec) error {
	if row.Timestamp < query.start || row.Timestamp > query.end {
		return fmt.Errorf("row timestamp %d outside query range [%d,%d]", row.Timestamp, query.start, query.end)
	}
	index, err := logicalIndexForTimestamp(row.Timestamp, query.timestampStep)
	if err != nil {
		return err
	}
	if index < query.startIndex || index > query.endIndex {
		return fmt.Errorf("row index %d outside query index range [%d,%d]", index, query.startIndex, query.endIndex)
	}
	if got := row.Tags["host"]; got != scaleHost(index) {
		return fmt.Errorf("row %d host = %q, want %q", index, got, scaleHost(index))
	}
	if len(row.Fields) != 10 {
		return fmt.Errorf("row %d field count = %d, want 10", index, len(row.Fields))
	}
	return validateScaleFields(index, row.Fields)
}

func expectedQueryRows(query querySpec) int {
	if query.endIndex < query.startIndex {
		return 0
	}
	available := query.endIndex - query.startIndex + 1
	if query.limit > 0 && query.limit < available {
		return query.limit
	}
	return available
}

func validateScaleFields(index int, fields map[string]mts.FieldValue) error {
	if err := validateScaleField(index, fields, "f0", mts.Float64Value(float64(index))); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "f1", mts.Float64Value(float64(index)*1.1)); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "f2", mts.Float64Value(float64(index)*1.2)); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "f3", mts.Float64Value(float64(index)*1.3)); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "f4", mts.Float64Value(float64(index)*1.4)); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "i0", mts.Int64Value(int64(index))); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "i1", mts.Int64Value(int64(index+1))); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "i2", mts.Int64Value(int64(index+2))); err != nil {
		return err
	}
	if err := validateScaleField(index, fields, "s0", mts.StringValue("ok")); err != nil {
		return err
	}
	return validateScaleField(index, fields, "b0", mts.BoolValue(index%2 == 0))
}

func validateScaleField(index int, fields map[string]mts.FieldValue, name string, want mts.FieldValue) error {
	if got := fields[name]; got != want {
		return fmt.Errorf("row %d field %s = %#v, want %#v", index, name, got, want)
	}
	return nil
}

func writeScaleBatches(ctx context.Context, eng *mts.Engine, cfg config) error {
	hostCache := scaleHostCache(100)
	builder := newScaleTypedBatchBuilder(cfg.batchSize)
	writeOptions := mts.WriteOptions{Sync: durabilityOptions(cfg.durability).writeSync}
	for start := 0; start < cfg.points; start += cfg.batchSize {
		end := start + cfg.batchSize
		if end > cfg.points {
			end = cfg.points
		}
		if cfg.ingestPath == "public" {
			if err := eng.Write(ctx, scalePoints(start, end, cfg), writeOptions); err != nil {
				return fmt.Errorf("write public batch: %w", err)
			}
			continue
		}
		if err := eng.WriteTypedBatch(ctx, builder.Build(start, end, hostCache, cfg), writeOptions); err != nil {
			return fmt.Errorf("write typed batch: %w", err)
		}
	}
	return nil
}

func scalePoints(start int, end int, cfg config) []mts.Point {
	points := make([]mts.Point, 0, end-start)
	step := timestampStepNanos(cfg)
	for index := start; index < end; index++ {
		points = append(points, mts.Point{
			Measurement: "scale",
			Tags:        map[string]string{"host": scaleHost(index)},
			Timestamp:   timestampForIndex(index, step),
			Fields: map[string]mts.FieldValue{
				"f0": mts.Float64Value(float64(index)),
				"f1": mts.Float64Value(float64(index) * 1.1),
				"f2": mts.Float64Value(float64(index) * 1.2),
				"f3": mts.Float64Value(float64(index) * 1.3),
				"f4": mts.Float64Value(float64(index) * 1.4),
				"i0": mts.Int64Value(int64(index)),
				"i1": mts.Int64Value(int64(index + 1)),
				"i2": mts.Int64Value(int64(index + 2)),
				"s0": mts.StringValue("ok"),
				"b0": mts.BoolValue(index%2 == 0),
			},
		})
	}
	return points
}

func scaleTypedBatch(start int, end int, hostCache []string) mts.TypedBatch {
	return newScaleTypedBatchBuilder(end-start).Build(start, end, hostCache, config{})
}

type scaleTypedBatchBuilder struct {
	timestamps []int64
	hosts      []string
	f0         []float64
	f1         []float64
	f2         []float64
	f3         []float64
	f4         []float64
	i0         []int64
	i1         []int64
	i2         []int64
	s0         []string
	b0         []bool
	batch      mts.TypedBatch
}

func newScaleTypedBatchBuilder(capacity int) *scaleTypedBatchBuilder {
	builder := &scaleTypedBatchBuilder{}
	builder.ensureCapacity(capacity)
	builder.batch = mts.TypedBatch{
		Measurement: "scale",
		Tags:        []mts.TagColumn{{Name: "host"}},
		Fields: []mts.TypedFieldColumn{
			{Name: "f0", Type: mts.FieldFloat64},
			{Name: "f1", Type: mts.FieldFloat64},
			{Name: "f2", Type: mts.FieldFloat64},
			{Name: "f3", Type: mts.FieldFloat64},
			{Name: "f4", Type: mts.FieldFloat64},
			{Name: "i0", Type: mts.FieldInt64},
			{Name: "i1", Type: mts.FieldInt64},
			{Name: "i2", Type: mts.FieldInt64},
			{Name: "s0", Type: mts.FieldString},
			{Name: "b0", Type: mts.FieldBool},
		},
	}
	return builder
}

func (b *scaleTypedBatchBuilder) Build(start int, end int, hostCache []string, cfg config) mts.TypedBatch {
	count := end - start
	b.ensureCapacity(count)
	b.resize(count)
	step := timestampStepNanos(cfg)
	for offset := range count {
		index := start + offset
		b.fillRow(offset, index, hostCache, step)
	}
	return b.batch
}

func (b *scaleTypedBatchBuilder) fillRow(offset int, index int, hostCache []string, timestampStep int64) {
	b.timestamps[offset] = timestampForIndex(index, timestampStep)
	b.hosts[offset] = hostCache[index%len(hostCache)]
	b.f0[offset] = float64(index)
	b.f1[offset] = float64(index) * 1.1
	b.f2[offset] = float64(index) * 1.2
	b.f3[offset] = float64(index) * 1.3
	b.f4[offset] = float64(index) * 1.4
	b.i0[offset] = int64(index)
	b.i1[offset] = int64(index + 1)
	b.i2[offset] = int64(index + 2)
	b.s0[offset] = "ok"
	b.b0[offset] = index%2 == 0
}

func (b *scaleTypedBatchBuilder) ensureCapacity(count int) {
	if cap(b.timestamps) >= count {
		return
	}
	b.timestamps = make([]int64, count)
	b.hosts = make([]string, count)
	b.f0 = make([]float64, count)
	b.f1 = make([]float64, count)
	b.f2 = make([]float64, count)
	b.f3 = make([]float64, count)
	b.f4 = make([]float64, count)
	b.i0 = make([]int64, count)
	b.i1 = make([]int64, count)
	b.i2 = make([]int64, count)
	b.s0 = make([]string, count)
	b.b0 = make([]bool, count)
}

func (b *scaleTypedBatchBuilder) resize(count int) {
	b.batch.Timestamps = b.timestamps[:count]
	b.batch.Tags[0].Values = b.hosts[:count]
	b.batch.Fields[0].Float64Values = b.f0[:count]
	b.batch.Fields[1].Float64Values = b.f1[:count]
	b.batch.Fields[2].Float64Values = b.f2[:count]
	b.batch.Fields[3].Float64Values = b.f3[:count]
	b.batch.Fields[4].Float64Values = b.f4[:count]
	b.batch.Fields[5].Int64Values = b.i0[:count]
	b.batch.Fields[6].Int64Values = b.i1[:count]
	b.batch.Fields[7].Int64Values = b.i2[:count]
	b.batch.Fields[8].StringValues = b.s0[:count]
	b.batch.Fields[9].BoolValues = b.b0[:count]
}

func scaleHostCache(count int) []string {
	hosts := make([]string, count)
	for index := range count {
		hosts[index] = scaleHost(index)
	}
	return hosts
}

func scaleHost(index int) string {
	return fmt.Sprintf("host-%03d", index%100)
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		total += info.Size()
		_ = path
		return nil
	})
	return total, err
}

func countSSTables(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}
		if strings.HasPrefix(info.Name(), "sst-") {
			count++
		}
		_ = path
		return nil
	})
	return count, err
}

func countShards(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == "shards" {
			count++
		}
		return nil
	})
	return count, err
}

func shardSSTableDistribution(root string) (map[string]int, error) {
	distribution := make(map[string]int)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == "shards" {
			distribution[info.Name()] = 0
			return nil
		}
		if strings.HasPrefix(info.Name(), "sst-") {
			shard := nearestShardDir(path)
			if shard != "" {
				distribution[shard]++
			}
		}
		return nil
	})
	return distribution, err
}

func nearestShardDir(path string) string {
	dir := filepath.Dir(path)
	for {
		if filepath.Base(filepath.Dir(dir)) == "shards" {
			return filepath.Base(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func levelDistribution(root string) (map[int]int, error) {
	levels := make(map[int]int)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || info.Name() != "MANIFEST.bin" {
			return err
		}
		manifest, err := sstable.LoadManifest(filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, part := range manifest.Parts {
			levels[part.Level]++
		}
		return nil
	})
	return levels, err
}

func captureSSTableSnapshot(root string) (sstableSnapshot, error) {
	count, err := countSSTables(root)
	if err != nil {
		return sstableSnapshot{}, err
	}
	levels, err := levelDistribution(root)
	if err != nil {
		return sstableSnapshot{}, err
	}
	return sstableSnapshot{count: count, levelDistribution: levels}, nil
}

func logicalInputBytes(points int) int64 {
	const wide10ValueBytes = int64(5*8 + 3*8 + 2 + 1 + 8)
	if points <= 0 {
		return 0
	}
	return int64(points) * wide10ValueBytes
}

func readAmplification(sstableCount int, rows int) float64 {
	if rows <= 0 {
		return 0
	}
	return float64(sstableCount)
}

func throughput(points int, duration time.Duration) float64 {
	if points <= 0 || duration <= 0 {
		return 0
	}
	return float64(points) / duration.Seconds()
}

func amplificationRatio(actual int64, logical int64) float64 {
	if actual <= 0 || logical <= 0 {
		return 0
	}
	return float64(actual) / float64(logical)
}

func compareBaseline(cfg config, out report) error {
	if cfg.baseline == "" {
		return nil
	}
	data, err := os.ReadFile(cfg.baseline)
	if err != nil {
		return fmt.Errorf("read baseline: %w", err)
	}
	var base report
	if err := json.Unmarshal(data, &base); err != nil {
		return fmt.Errorf("decode baseline: %w", err)
	}
	limit := cfg.maxRegressionPercent
	if regressionPercent(float64(out.Duration), float64(base.Duration)) > limit {
		return fmt.Errorf("duration regression %.2f%% exceeds %.2f%%", regressionPercent(float64(out.Duration), float64(base.Duration)), limit)
	}
	if regressionPercent(float64(out.DataBytes), float64(base.DataBytes)) > limit {
		return fmt.Errorf("data bytes regression %.2f%% exceeds %.2f%%", regressionPercent(float64(out.DataBytes), float64(base.DataBytes)), limit)
	}
	if regressionPercent(float64(out.HeapAlloc), float64(base.HeapAlloc)) > limit {
		return fmt.Errorf("heap alloc regression %.2f%% exceeds %.2f%%", regressionPercent(float64(out.HeapAlloc), float64(base.HeapAlloc)), limit)
	}
	return nil
}

func enforceThresholds(cfg config, out report) error {
	violations := make([]string, 0, 3)
	if cfg.maxRSSBytes > 0 && out.RSSPeakBytes > cfg.maxRSSBytes {
		violations = append(violations, thresholdMessage("rss_peak_bytes", out.RSSPeakBytes, cfg.maxRSSBytes))
	}
	if cfg.maxSSTableCount > 0 && out.SSTableCount > cfg.maxSSTableCount {
		violations = append(violations, thresholdMessage("sstable_count", int64(out.SSTableCount), int64(cfg.maxSSTableCount)))
	}
	if cfg.maxCompactionBacklog > 0 && out.CompactionStats.Backlog > cfg.maxCompactionBacklog {
		violations = append(violations, thresholdMessage(
			"compaction_backlog",
			int64(out.CompactionStats.Backlog),
			int64(cfg.maxCompactionBacklog),
		))
	}
	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("scale thresholds exceeded: %s", strings.Join(violations, "; "))
}

func thresholdMessage(name string, actual int64, limit int64) string {
	return fmt.Sprintf("%s=%d limit=%d", name, actual, limit)
}

func regressionPercent(actual float64, baseline float64) float64 {
	if baseline <= 0 || actual <= baseline {
		return 0
	}
	return (actual - baseline) / baseline * 100
}
