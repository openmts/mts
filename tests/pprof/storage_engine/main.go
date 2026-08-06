package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	mts "github.com/openmts/mts"
)

type config struct {
	dataDir                      string
	mode                         string
	ingestPath                   string
	fieldLayout                  string
	points                       int
	series                       int
	queryRepeat                  int
	writeBatchSize               int
	memTableMaxSamples           int
	storageSoftBytesLimit        int64
	storageHardBytesLimit        int64
	storageQueryBytesLimit       int64
	storageFlushBytesLimit       int64
	storageCompactionBytesLimit  int64
	storageCompressionBytesLimit int64
	compactionEnabled            bool
	compactionLevel0PartLimit    int
	compactionLevel0SizeLimit    int64
	compactionMaxOutputPartBytes int64
	compactionLevelsSpec         string
	compactionLevels             []mts.CompactionLevelOptions
	compactionMaxCascadeSteps    int
	compactionBackgroundInterval time.Duration
	compressionAlgorithm         string
	flushOnExit                  bool
	cpuProfile                   string
	memProfile                   string
	prebuildPoints               bool
	prebuilt                     []mts.Point
}

const (
	fieldLayoutDefault        = "default"
	fieldLayoutWide10         = "wide10"
	ingestPathPublic          = "public"
	ingestPathTyped           = "typed"
	compressionOff            = "off"
	compressionNone           = "none"
	defaultWriteBatchSize     = 1024
	defaultMemTableMaxSamples = 8192
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("storage_engine pprof failed: %v", err)
	}
}

func run(args []string) (err error) {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if cfg.prebuildPoints {
		rate := runtime.MemProfileRate
		runtime.MemProfileRate = 0
		cfg.prebuilt = buildWorkloadPoints(cfg)
		runtime.GC()
		runtime.MemProfileRate = rate
		if err := logStageMetrics("after_prebuild", ""); err != nil {
			return err
		}
	}
	dir, cleanup, err := prepareDataDir(cfg.dataDir)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()
	stopCPU, err := startCPUProfile(cfg.cpuProfile)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, stopCPU())
	}()

	ctx := context.Background()
	eng, err := mts.Open(ctx, storageOptions(dir, cfg))
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()

	if err := logStageMetrics("before_workload", dir); err != nil {
		return err
	}
	workloadStarted := time.Now()
	if err := runWorkloadWithDir(ctx, eng, cfg, dir); err != nil {
		return err
	}
	workloadDuration := time.Since(workloadStarted)
	if cfg.flushOnExit && cfg.mode != "replay" {
		if err := eng.Flush(ctx); err != nil {
			return fmt.Errorf("flush on exit: %w", err)
		}
	}
	if err := logStageMetrics("after_workload", dir); err != nil {
		return err
	}
	if err := writeMemProfile(cfg.memProfile); err != nil {
		return err
	}
	if err := logStageMetrics("after_profile", dir); err != nil {
		return err
	}
	metrics, err := collectRunMetrics(dir)
	if err != nil {
		return err
	}
	storageMemory := eng.StorageMemorySnapshot()
	log.Printf(
		"mode=%s field_layout=%s points=%d series=%d query_repeat=%d write_batch_size=%d memtable_max_samples=%d compaction_enabled=%t compression_algorithm=%s data_dir=%s",
		cfg.mode,
		normalizedFieldLayout(cfg.fieldLayout),
		cfg.points,
		cfg.series,
		cfg.queryRepeat,
		normalizedWriteBatchSize(cfg),
		normalizedMemTableMaxSamples(cfg),
		cfg.compactionEnabled,
		normalizedCompressionAlgorithm(cfg.compressionAlgorithm),
		dir,
	)
	log.Printf(
		"metrics workload_duration_ms=%d sstable_count=%d data_dir_bytes=%d heap_alloc_bytes=%d heap_sys_bytes=%d heap_total_alloc_bytes=%d mallocs=%d frees=%d num_gc=%d pause_total_ns=%d rss_bytes=%d rss_peak_bytes=%d storage_current_bytes=%d storage_peak_bytes=%d storage_active_bytes=%d storage_memtable_bytes=%d storage_wal_bytes=%d storage_reservation_bytes=%d storage_write_bytes=%d storage_flush_bytes=%d storage_query_bytes=%d storage_compaction_bytes=%d storage_compression_bytes=%d storage_rejected_writes=%d storage_rejected_reservations=%d storage_flush_triggered=%d storage_runtime_heap_alloc_bytes=%d storage_runtime_rss_bytes=%d storage_runtime_gap_bytes=%d",
		workloadDuration.Milliseconds(),
		metrics.sstableCount,
		metrics.dataDirBytes,
		metrics.heapAllocBytes,
		metrics.heapSysBytes,
		metrics.heapTotalAllocBytes,
		metrics.mallocs,
		metrics.frees,
		metrics.numGC,
		metrics.pauseTotalNs,
		metrics.rssBytes,
		metrics.rssPeakBytes,
		storageMemory.CurrentBytes,
		storageMemory.PeakBytes,
		storageMemory.ActiveBytes,
		storageMemory.MemTableBytes,
		storageMemory.WALBytes,
		storageMemory.ReservationBytes,
		storageMemory.WriteBytes,
		storageMemory.FlushBytes,
		storageMemory.QueryBytes,
		storageMemory.CompactionBytes,
		storageMemory.CompressionBytes,
		storageMemory.RejectedWrites,
		storageMemory.RejectedReservations,
		storageMemory.FlushTriggered,
		storageMemory.RuntimeHeapAllocBytes,
		storageMemory.RuntimeRSSBytes,
		storageMemory.RuntimeGapBytes,
	)
	return nil
}

func logStageMetrics(stage string, dir string) error {
	metrics, err := collectRunMetrics(dir)
	if err != nil {
		return err
	}
	log.Printf(
		"stage=%s data_dir_bytes=%d heap_alloc_bytes=%d heap_sys_bytes=%d heap_total_alloc_bytes=%d mallocs=%d frees=%d num_gc=%d rss_bytes=%d rss_peak_bytes=%d",
		stage,
		metrics.dataDirBytes,
		metrics.heapAllocBytes,
		metrics.heapSysBytes,
		metrics.heapTotalAllocBytes,
		metrics.mallocs,
		metrics.frees,
		metrics.numGC,
		metrics.rssBytes,
		metrics.rssPeakBytes,
	)
	return nil
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("storage_engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&cfg.dataDir, "data-dir", "", "数据目录；为空时使用临时目录并自动清理")
	flags.StringVar(&cfg.mode, "mode", "query", "workload 模式：write/query/compact/replay")
	flags.StringVar(&cfg.ingestPath, "ingest-path", ingestPathTyped, "写入路径：typed/public")
	flags.StringVar(&cfg.fieldLayout, "field-layout", fieldLayoutDefault, "字段布局：default/wide10")
	flags.IntVar(&cfg.points, "points", 10000, "写入点数")
	flags.IntVar(&cfg.series, "series", 100, "series 数量")
	flags.IntVar(&cfg.queryRepeat, "query-repeat", 5, "查询重复次数")
	flags.IntVar(&cfg.writeBatchSize, "write-batch-size", defaultWriteBatchSize, "每次 Engine.Write 提交的 point 数")
	flags.IntVar(&cfg.memTableMaxSamples, "memtable-max-samples", defaultMemTableMaxSamples, "触发 MemTable flush 的 sample 数；wide10 中 1 point=10 samples")
	flags.Int64Var(&cfg.storageSoftBytesLimit, "storage-soft-bytes-limit", 0, "存储层软字节阈值；达到后触发 flush")
	flags.Int64Var(&cfg.storageHardBytesLimit, "storage-hard-bytes-limit", 0, "存储层硬字节阈值；超过后拒绝写入")
	flags.Int64Var(&cfg.storageQueryBytesLimit, "storage-query-bytes-limit", 0, "查询物化字节阈值；超过后返回错误")
	flags.Int64Var(&cfg.storageFlushBytesLimit, "storage-flush-bytes-limit", 0, "flush 临时字节阈值；超过后返回错误")
	flags.Int64Var(&cfg.storageCompactionBytesLimit, "storage-compaction-bytes-limit", 0, "compaction 临时字节阈值；超过后返回错误")
	flags.Int64Var(&cfg.storageCompressionBytesLimit, "storage-compression-bytes-limit", 0, "payload compression 临时字节阈值；超过后返回错误")
	flags.BoolVar(&cfg.compactionEnabled, "compaction-enabled", false, "启用 size-tiered compaction")
	flags.IntVar(&cfg.compactionLevel0PartLimit, "compaction-level0-part-limit", 0, "Level-0 part 数超过该值时触发 compaction；0 使用引擎默认值")
	flags.Int64Var(&cfg.compactionLevel0SizeLimit, "compaction-level0-size-limit", 0, "Level-0 part 总大小超过该值时触发 compaction；0 表示不按大小触发")
	flags.Int64Var(&cfg.compactionMaxOutputPartBytes, "compaction-max-output-part-bytes", 0, "compaction 输出 part 目标大小；0 表示单 part 输出")
	flags.StringVar(&cfg.compactionLevelsSpec, "compaction-levels", "", "逐层 compaction 配置，格式 level:part_limit:size_limit:max_output_bytes，多层用逗号分隔")
	flags.IntVar(&cfg.compactionMaxCascadeSteps, "compaction-max-cascade-steps", 0, "单次触发允许的最大级联 compaction 步数；0 使用引擎默认值")
	flags.DurationVar(&cfg.compactionBackgroundInterval, "compaction-background-interval", 0, "后台 compaction 间隔；0 表示不启动后台循环")
	flags.StringVar(&cfg.compressionAlgorithm, "compression-algorithm", compressionOff, "压缩算法：off/none/snappy/lz4/zstd；off 完全关闭，none 仅启用 typed encoding")
	flags.BoolVar(&cfg.flushOnExit, "flush-on-exit", false, "workload 结束后强制 Flush，便于统计完整落盘后的 SSTable 数量")
	flags.StringVar(&cfg.cpuProfile, "cpu-profile", "", "CPU profile 输出文件")
	flags.StringVar(&cfg.memProfile, "mem-profile", "", "heap profile 输出文件")
	flags.BoolVar(&cfg.prebuildPoints, "prebuild-points", false, "在 profile 主阶段前预生成 points")
	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}
	levels, err := parseCompactionLevels(cfg.compactionLevelsSpec)
	if err != nil {
		return config{}, err
	}
	cfg.compactionLevels = levels
	return cfg, nil
}

func validateConfig(cfg config) error {
	switch cfg.mode {
	case "write", "query", "compact", "replay", "read":
	default:
		return fmt.Errorf("unsupported mode %q", cfg.mode)
	}
	if cfg.points <= 0 {
		return fmt.Errorf("points must be positive")
	}
	if cfg.series <= 0 {
		return fmt.Errorf("series must be positive")
	}
	if cfg.series > cfg.points {
		return fmt.Errorf("series must be less than or equal to points")
	}
	if cfg.queryRepeat < 0 {
		return fmt.Errorf("query-repeat must be non-negative")
	}
	if cfg.writeBatchSize <= 0 {
		return fmt.Errorf("write-batch-size must be positive")
	}
	if cfg.memTableMaxSamples <= 0 {
		return fmt.Errorf("memtable-max-samples must be positive")
	}
	if cfg.storageSoftBytesLimit < 0 {
		return fmt.Errorf("storage-soft-bytes-limit must be non-negative")
	}
	if cfg.storageHardBytesLimit < 0 {
		return fmt.Errorf("storage-hard-bytes-limit must be non-negative")
	}
	if cfg.storageQueryBytesLimit < 0 {
		return fmt.Errorf("storage-query-bytes-limit must be non-negative")
	}
	if cfg.storageFlushBytesLimit < 0 {
		return fmt.Errorf("storage-flush-bytes-limit must be non-negative")
	}
	if cfg.storageCompactionBytesLimit < 0 {
		return fmt.Errorf("storage-compaction-bytes-limit must be non-negative")
	}
	if cfg.storageCompressionBytesLimit < 0 {
		return fmt.Errorf("storage-compression-bytes-limit must be non-negative")
	}
	if cfg.compactionLevel0PartLimit < 0 {
		return fmt.Errorf("compaction-level0-part-limit must be non-negative")
	}
	if cfg.compactionLevel0SizeLimit < 0 {
		return fmt.Errorf("compaction-level0-size-limit must be non-negative")
	}
	if cfg.compactionMaxOutputPartBytes < 0 {
		return fmt.Errorf("compaction-max-output-part-bytes must be non-negative")
	}
	if cfg.compactionMaxCascadeSteps < 0 {
		return fmt.Errorf("compaction-max-cascade-steps must be non-negative")
	}
	if cfg.compactionBackgroundInterval < 0 {
		return fmt.Errorf("compaction-background-interval must be non-negative")
	}
	switch normalizedFieldLayout(cfg.fieldLayout) {
	case fieldLayoutDefault, fieldLayoutWide10:
	default:
		return fmt.Errorf("unsupported field layout %q", cfg.fieldLayout)
	}
	switch normalizedIngestPath(cfg.ingestPath) {
	case ingestPathPublic, ingestPathTyped:
	default:
		return fmt.Errorf("unsupported ingest path %q", cfg.ingestPath)
	}
	switch normalizedCompressionAlgorithm(cfg.compressionAlgorithm) {
	case compressionOff, compressionNone, "snappy", "lz4", "zstd":
	default:
		return fmt.Errorf("unsupported compression algorithm %q", cfg.compressionAlgorithm)
	}
	return nil
}

func parseCompactionLevels(spec string) ([]mts.CompactionLevelOptions, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	items := strings.Split(spec, ",")
	levels := make([]mts.CompactionLevelOptions, 0, len(items))
	for _, item := range items {
		level, err := parseCompactionLevel(item)
		if err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}
	return levels, nil
}

func parseCompactionLevel(item string) (mts.CompactionLevelOptions, error) {
	fields := strings.Split(strings.TrimSpace(item), ":")
	if len(fields) != 4 {
		return mts.CompactionLevelOptions{}, fmt.Errorf("invalid compaction level %q", item)
	}
	level, err := parseNonNegativeInt(fields[0], "compaction level")
	if err != nil {
		return mts.CompactionLevelOptions{}, err
	}
	partLimit, err := parseNonNegativeInt(fields[1], "compaction part limit")
	if err != nil {
		return mts.CompactionLevelOptions{}, err
	}
	sizeLimit, err := parseNonNegativeInt64(fields[2], "compaction size limit")
	if err != nil {
		return mts.CompactionLevelOptions{}, err
	}
	maxOutputBytes, err := parseNonNegativeInt64(fields[3], "compaction max output bytes")
	if err != nil {
		return mts.CompactionLevelOptions{}, err
	}
	return mts.CompactionLevelOptions{
		Level:              level,
		PartLimit:          partLimit,
		SizeLimit:          sizeLimit,
		MaxOutputPartBytes: maxOutputBytes,
	}, nil
}

func parseNonNegativeInt(value string, name string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return parsed, nil
}

func parseNonNegativeInt64(value string, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be non-negative", name)
	}
	return parsed, nil
}

func normalizedFieldLayout(layout string) string {
	if layout == "" {
		return fieldLayoutDefault
	}
	return layout
}

func normalizedIngestPath(path string) string {
	if path == "" {
		return ingestPathTyped
	}
	return path
}

func storageOptions(path string, cfg config) mts.Options {
	return mts.Options{
		Path:               path,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: normalizedMemTableMaxSamples(cfg),
		Compaction: mts.CompactionOptions{
			Enabled:            cfg.compactionEnabled,
			Level0PartLimit:    cfg.compactionLevel0PartLimit,
			Level0SizeLimit:    cfg.compactionLevel0SizeLimit,
			MaxOutputPartBytes: cfg.compactionMaxOutputPartBytes,
			Levels:             cfg.compactionLevels,
			MaxCascadeSteps:    cfg.compactionMaxCascadeSteps,
			BackgroundInterval: cfg.compactionBackgroundInterval,
		},
		Compression: compressionOptions(cfg.compressionAlgorithm),
		StorageMemory: mts.StorageMemoryOptions{
			SoftBytesLimit:        cfg.storageSoftBytesLimit,
			HardBytesLimit:        cfg.storageHardBytesLimit,
			QueryBytesLimit:       cfg.storageQueryBytesLimit,
			FlushBytesLimit:       cfg.storageFlushBytesLimit,
			CompactionBytesLimit:  cfg.storageCompactionBytesLimit,
			CompressionBytesLimit: cfg.storageCompressionBytesLimit,
		},
	}
}

func compressionOptions(algorithm string) mts.CompressionOptions {
	normalized := normalizedCompressionAlgorithm(algorithm)
	if normalized == compressionOff {
		return mts.CompressionOptions{}
	}
	return mts.CompressionOptions{
		Enabled:       true,
		Algorithm:     normalized,
		MinPageValues: 1,
	}
}

func normalizedCompressionAlgorithm(algorithm string) string {
	if algorithm == "" {
		return compressionOff
	}
	return algorithm
}

func normalizedWriteBatchSize(cfg config) int {
	if cfg.writeBatchSize <= 0 {
		return defaultWriteBatchSize
	}
	return cfg.writeBatchSize
}

func normalizedMemTableMaxSamples(cfg config) int {
	if cfg.memTableMaxSamples <= 0 {
		return defaultMemTableMaxSamples
	}
	return cfg.memTableMaxSamples
}

func prepareDataDir(path string) (string, func() error, error) {
	if path != "" {
		clean := filepath.Clean(path)
		if err := os.MkdirAll(clean, 0700); err != nil {
			return "", nil, fmt.Errorf("create data dir: %w", err)
		}
		if err := os.Chmod(clean, 0700); err != nil {
			return "", nil, fmt.Errorf("set data dir permissions: %w", err)
		}
		return clean, func() error { return nil }, nil
	}
	dir, err := os.MkdirTemp("", "mts-pprof-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp data dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("set temp data dir permissions: %w", err)
	}
	return dir, func() error {
		return os.RemoveAll(dir)
	}, nil
}

type runMetrics struct {
	sstableCount        int
	dataDirBytes        int64
	heapAllocBytes      uint64
	heapSysBytes        uint64
	heapTotalAllocBytes uint64
	mallocs             uint64
	frees               uint64
	numGC               uint32
	pauseTotalNs        uint64
	rssBytes            uint64
	rssPeakBytes        uint64
}

func collectRunMetrics(root string) (runMetrics, error) {
	var metrics runMetrics
	if root != "" {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if shouldIgnoreWalkError(root, path, walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), "sst-") {
					metrics.sstableCount++
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				if shouldIgnoreWalkError(root, path, err) {
					return nil
				}
				return err
			}
			metrics.dataDirBytes += info.Size()
			return nil
		})
		if err != nil {
			return runMetrics{}, fmt.Errorf("collect run metrics: %w", err)
		}
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	metrics.heapAllocBytes = mem.HeapAlloc
	metrics.heapSysBytes = mem.HeapSys
	metrics.heapTotalAllocBytes = mem.TotalAlloc
	metrics.mallocs = mem.Mallocs
	metrics.frees = mem.Frees
	metrics.numGC = mem.NumGC
	metrics.pauseTotalNs = mem.PauseTotalNs
	metrics.rssBytes, metrics.rssPeakBytes = readProcessRSS()
	return metrics, nil
}

func shouldIgnoreWalkError(root, path string, err error) bool {
	return path != root && os.IsNotExist(err)
}

func readProcessRSS() (uint64, uint64) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, 0
	}
	rss, peak, err := parseProcStatusRSS(string(data))
	if err != nil {
		return 0, 0
	}
	return rss, peak
}

func parseProcStatusRSS(status string) (uint64, uint64, error) {
	var rss uint64
	var peak uint64
	for _, line := range strings.Split(status, "\n") {
		switch {
		case strings.HasPrefix(line, "VmRSS:"):
			value, err := parseProcStatusKB(line)
			if err != nil {
				return 0, 0, err
			}
			rss = value
		case strings.HasPrefix(line, "VmHWM:"):
			value, err := parseProcStatusKB(line)
			if err != nil {
				return 0, 0, err
			}
			peak = value
		}
	}
	return rss, peak, nil
}

func parseProcStatusKB(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("parse proc status line %q: missing value", line)
	}
	kb, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse proc status line %q: %w", line, err)
	}
	return kb * 1024, nil
}

func startCPUProfile(path string) (func() error, error) {
	if path == "" {
		return func() error { return nil }, nil
	}
	file, err := createProfileFile(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("start CPU profile: %w", err), closeErr)
	}
	return func() error {
		pprof.StopCPUProfile()
		if err := file.Close(); err != nil {
			return fmt.Errorf("close CPU profile: %w", err)
		}
		return nil
	}, nil
}

func runWorkload(ctx context.Context, eng *mts.Engine, cfg config) error {
	return runWorkloadWithDir(ctx, eng, cfg, "")
}

func runWorkloadWithDir(ctx context.Context, eng *mts.Engine, cfg config, dir string) error {
	switch cfg.mode {
	case "write":
		return writePoints(ctx, eng, cfg)
	case "read":
		return queryRows(ctx, eng, cfg)
	case "query":
		return queryWorkload(ctx, eng, cfg)
	case "compact":
		return compactWorkload(ctx, eng, cfg)
	case "replay":
		return replayWorkload(ctx, eng, cfg, dir)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.mode)
	}
}

func queryWorkload(ctx context.Context, eng *mts.Engine, cfg config) error {
	if err := writePoints(ctx, eng, cfg); err != nil {
		return err
	}
	if err := eng.Flush(ctx); err != nil {
		return fmt.Errorf("flush engine: %w", err)
	}
	if err := eng.Compact(ctx); err != nil {
		return fmt.Errorf("compact engine: %w", err)
	}
	if err := queryRows(ctx, eng, cfg); err != nil {
		return err
	}
	return nil
}

func compactWorkload(ctx context.Context, eng *mts.Engine, cfg config) error {
	flushEvery := max(cfg.points/4, 1)
	for index := range cfg.points {
		point := workloadPointAt(cfg, index)
		if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{}); err != nil {
			return fmt.Errorf("write compact point: %w", err)
		}
		if (index+1)%flushEvery == 0 {
			if err := eng.Flush(ctx); err != nil {
				return fmt.Errorf("flush compact batch: %w", err)
			}
		}
	}
	if err := eng.Flush(ctx); err != nil {
		return fmt.Errorf("flush compact final: %w", err)
	}
	if err := eng.Compact(ctx); err != nil {
		return fmt.Errorf("compact engine: %w", err)
	}
	return nil
}

func replayWorkload(ctx context.Context, eng *mts.Engine, cfg config, dir string) error {
	if dir == "" {
		return fmt.Errorf("replay mode requires data dir")
	}
	if err := writePointsSynced(ctx, eng, cfg); err != nil {
		return err
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close before replay: %w", err)
	}
	reopened, err := mts.Open(ctx, storageOptions(dir, cfg))
	if err != nil {
		return fmt.Errorf("reopen for replay: %w", err)
	}
	if err := queryRows(ctx, reopened, cfg); err != nil {
		closeErr := reopened.Close(ctx)
		return errors.Join(err, closeErr)
	}
	if err := reopened.Close(ctx); err != nil {
		return fmt.Errorf("close replay engine: %w", err)
	}
	return nil
}

func writePoints(ctx context.Context, eng *mts.Engine, cfg config) error {
	if normalizedIngestPath(cfg.ingestPath) == ingestPathTyped {
		return writeTypedPoints(ctx, eng, cfg, false)
	}
	batchSize := normalizedWriteBatchSize(cfg)
	batch := make([]mts.Point, 0, batchSize)
	for index := range cfg.points {
		batch = append(batch, workloadPointAt(cfg, index))
		if len(batch) == batchSize {
			if err := eng.Write(ctx, batch, mts.WriteOptions{}); err != nil {
				return fmt.Errorf("write batch: %w", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) == 0 {
		return nil
	}
	if err := eng.Write(ctx, batch, mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write final batch: %w", err)
	}
	return nil
}

func writePointsSynced(ctx context.Context, eng *mts.Engine, cfg config) error {
	if normalizedIngestPath(cfg.ingestPath) == ingestPathTyped {
		return writeTypedPoints(ctx, eng, cfg, true)
	}
	batchSize := normalizedWriteBatchSize(cfg)
	batch := make([]mts.Point, 0, batchSize)
	for index := range cfg.points {
		batch = append(batch, workloadPointAt(cfg, index))
		if len(batch) == batchSize {
			if err := eng.Write(ctx, batch, mts.WriteOptions{Sync: true}); err != nil {
				return fmt.Errorf("write synced batch: %w", err)
			}
			batch = batch[:0]
		}
	}
	if len(batch) == 0 {
		return nil
	}
	if err := eng.Write(ctx, batch, mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write final synced batch: %w", err)
	}
	return nil
}

func writeTypedPoints(ctx context.Context, eng *mts.Engine, cfg config, syncWrite bool) error {
	batchSize := normalizedWriteBatchSize(cfg)
	hostCache := workloadHostCache(cfg.series)
	for start := 0; start < cfg.points; start += batchSize {
		end := start + batchSize
		if end > cfg.points {
			end = cfg.points
		}
		if err := eng.WriteTypedBatch(ctx, typedWorkloadBatch(start, end, cfg, hostCache), mts.WriteOptions{Sync: syncWrite}); err != nil {
			return fmt.Errorf("write typed batch: %w", err)
		}
	}
	return nil
}

func buildWorkloadPoints(cfg config) []mts.Point {
	points := make([]mts.Point, cfg.points)
	for index := range cfg.points {
		points[index] = workloadPoint(index, cfg.series, cfg.fieldLayout)
	}
	return points
}

func typedWorkloadBatch(start int, end int, cfg config, hostCache []string) mts.TypedBatch {
	if normalizedFieldLayout(cfg.fieldLayout) == fieldLayoutWide10 {
		return wide10TypedWorkloadBatch(start, end, hostCache)
	}
	return defaultTypedWorkloadBatch(start, end, hostCache)
}

func defaultTypedWorkloadBatch(start int, end int, hostCache []string) mts.TypedBatch {
	count := end - start
	timestamps, hosts := typedBatchIdentityColumns(start, end, hostCache)
	active := make([]bool, count)
	countValues := make([]int64, count)
	states := make([]string, count)
	values := make([]float64, count)
	for offset := range count {
		index := start + offset
		active[offset] = index%2 == 0
		countValues[offset] = int64(index)
		states[offset] = "ok"
		values[offset] = float64(index)
	}
	return mts.TypedBatch{
		Measurement: "pprof",
		Tags:        []mts.TagColumn{{Name: "host", Values: hosts}},
		Timestamps:  timestamps,
		Fields: []mts.TypedFieldColumn{
			{Name: "active", Type: mts.FieldBool, BoolValues: active},
			{Name: "count", Type: mts.FieldInt64, Int64Values: countValues},
			{Name: "state", Type: mts.FieldString, StringValues: states},
			{Name: "value", Type: mts.FieldFloat64, Float64Values: values},
		},
	}
}

func wide10TypedWorkloadBatch(start int, end int, hostCache []string) mts.TypedBatch {
	count := end - start
	timestamps, hosts := typedBatchIdentityColumns(start, end, hostCache)
	active := make([]bool, count)
	f0 := make([]float64, count)
	f1 := make([]float64, count)
	f2 := make([]float64, count)
	f3 := make([]float64, count)
	f4 := make([]float64, count)
	i0 := make([]int64, count)
	i1 := make([]int64, count)
	i2 := make([]int64, count)
	states := make([]string, count)
	for offset := range count {
		index := start + offset
		seriesID := index % len(hostCache)
		active[offset] = index%2 == 0
		f0[offset] = float64(index)
		f1[offset] = float64(index) + 0.1
		f2[offset] = float64(index) + 0.2
		f3[offset] = float64(index) + 0.3
		f4[offset] = float64(index) + 0.4
		i0[offset] = int64(index)
		i1[offset] = int64(seriesID)
		i2[offset] = int64(index % 86400)
		states[offset] = "ok"
	}
	return mts.TypedBatch{
		Measurement: "pprof",
		Tags:        []mts.TagColumn{{Name: "host", Values: hosts}},
		Timestamps:  timestamps,
		Fields: []mts.TypedFieldColumn{
			{Name: "active", Type: mts.FieldBool, BoolValues: active},
			{Name: "f0", Type: mts.FieldFloat64, Float64Values: f0},
			{Name: "f1", Type: mts.FieldFloat64, Float64Values: f1},
			{Name: "f2", Type: mts.FieldFloat64, Float64Values: f2},
			{Name: "f3", Type: mts.FieldFloat64, Float64Values: f3},
			{Name: "f4", Type: mts.FieldFloat64, Float64Values: f4},
			{Name: "i0", Type: mts.FieldInt64, Int64Values: i0},
			{Name: "i1", Type: mts.FieldInt64, Int64Values: i1},
			{Name: "i2", Type: mts.FieldInt64, Int64Values: i2},
			{Name: "state", Type: mts.FieldString, StringValues: states},
		},
	}
}

func typedBatchIdentityColumns(start int, end int, hostCache []string) ([]int64, []string) {
	count := end - start
	timestamps := make([]int64, count)
	hosts := make([]string, count)
	for offset := range count {
		index := start + offset
		timestamps[offset] = int64(index)
		hosts[offset] = hostCache[index%len(hostCache)]
	}
	return timestamps, hosts
}

func workloadHostCache(series int) []string {
	hosts := make([]string, series)
	for index := range series {
		hosts[index] = fmt.Sprintf("host-%04d", index)
	}
	return hosts
}

func workloadPointAt(cfg config, index int) mts.Point {
	if len(cfg.prebuilt) == cfg.points {
		return cfg.prebuilt[index]
	}
	return workloadPoint(index, cfg.series, cfg.fieldLayout)
}

func workloadPoint(index int, series int, layout string) mts.Point {
	if normalizedFieldLayout(layout) == fieldLayoutWide10 {
		return wide10WorkloadPoint(index, series)
	}
	return defaultWorkloadPoint(index, series)
}

func defaultWorkloadPoint(index int, series int) mts.Point {
	seriesID := index % series
	return mts.Point{
		Measurement: "pprof",
		Tags:        map[string]string{"host": fmt.Sprintf("host-%04d", seriesID)},
		Timestamp:   int64(index),
		Fields: map[string]mts.FieldValue{
			"active": mts.BoolValue(index%2 == 0),
			"count":  mts.Int64Value(int64(index)),
			"state":  mts.StringValue("ok"),
			"value":  mts.Float64Value(float64(index)),
		},
	}
}

func wide10WorkloadPoint(index int, series int) mts.Point {
	seriesID := index % series
	return mts.Point{
		Measurement: "pprof",
		Tags:        map[string]string{"host": fmt.Sprintf("host-%04d", seriesID)},
		Timestamp:   int64(index),
		Fields: map[string]mts.FieldValue{
			"active": mts.BoolValue(index%2 == 0),
			"f0":     mts.Float64Value(float64(index)),
			"f1":     mts.Float64Value(float64(index) + 0.1),
			"f2":     mts.Float64Value(float64(index) + 0.2),
			"f3":     mts.Float64Value(float64(index) + 0.3),
			"f4":     mts.Float64Value(float64(index) + 0.4),
			"i0":     mts.Int64Value(int64(index)),
			"i1":     mts.Int64Value(int64(seriesID)),
			"i2":     mts.Int64Value(int64(index % 86400)),
			"state":  mts.StringValue("ok"),
		},
	}
}

func queryRows(ctx context.Context, eng *mts.Engine, cfg config) error {
	var totalStats mts.QueryStats
	totalRows := 0
	for repeat := range cfg.queryRepeat {
		host := fmt.Sprintf("host-%04d", repeat%cfg.series)
		iter, err := eng.QueryRowIterator(ctx, mts.Query{
			Measurement: "pprof",
			Tags:        map[string]string{"host": host},
			StartTime:   0,
			EndTime:     int64(cfg.points),
		})
		if err != nil {
			return fmt.Errorf("query rows: %w", err)
		}
		rows, err := drainRowIterator(iter)
		if err != nil {
			return fmt.Errorf("query rows: %w", err)
		}
		if rows == 0 {
			return fmt.Errorf("query host %s returned no rows", host)
		}
		totalRows += rows
		totalStats = mergeQueryStats(totalStats, eng.QueryStatsSnapshot())
	}
	log.Printf(
		"query_stats query_rows_streamed=%d query_stats_samples=%d query_stats_value_pages=%d query_stats_parts=%d query_stats_errors=%d",
		totalRows,
		totalStats.SamplesReturned,
		totalStats.ValuePagesRead,
		totalStats.PartsScanned,
		totalStats.Errors,
	)
	return nil
}

func drainRowIterator(iter mts.RowIterator) (int, error) {
	rows := 0
	for iter.Next() {
		rows++
	}
	err := errors.Join(iter.Err(), iter.Close())
	return rows, err
}

func mergeQueryStats(left mts.QueryStats, right mts.QueryStats) mts.QueryStats {
	left.CandidateShards += right.CandidateShards
	left.ShardsScanned += right.ShardsScanned
	left.ShardsSkipped += right.ShardsSkipped
	left.PartsScanned += right.PartsScanned
	left.PartsSkipped += right.PartsSkipped
	left.IndexRowsRead += right.IndexRowsRead
	left.IndexRowsSkipped += right.IndexRowsSkipped
	left.TimeBlocksRead += right.TimeBlocksRead
	left.ValueBlocksRead += right.ValueBlocksRead
	left.ValuePagesRead += right.ValuePagesRead
	left.ValuePagesSkipped += right.ValuePagesSkipped
	left.SamplesRead += right.SamplesRead
	left.SamplesReturned += right.SamplesReturned
	left.Errors += right.Errors
	return left
}

func writeMemProfile(path string) error {
	if path == "" {
		return nil
	}
	file, err := createProfileFile(path)
	if err != nil {
		return err
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(file); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write heap profile: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close heap profile: %w", err)
	}
	return nil
}

func createProfileFile(path string) (*os.File, error) {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, fmt.Errorf("set profile dir permissions: %w", err)
	}
	file, err := os.OpenFile(clean, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("create profile file: %w", err)
	}
	return file, nil
}
