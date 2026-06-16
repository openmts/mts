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

	mts "codeberg.org/mts/mts"
)

type config struct {
	dataDir                      string
	mode                         string
	fieldLayout                  string
	points                       int
	series                       int
	queryRepeat                  int
	writeBatchSize               int
	memTableMaxSamples           int
	compactionEnabled            bool
	compactionLevel0PartLimit    int
	compactionLevel0SizeLimit    int64
	compactionMaxOutputPartBytes int64
	compactionBackgroundInterval time.Duration
	flushOnExit                  bool
	cpuProfile                   string
	memProfile                   string
	prebuildPoints               bool
	prebuilt                     []mts.Point
}

const (
	fieldLayoutDefault        = "default"
	fieldLayoutWide10         = "wide10"
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

	if err := runWorkloadWithDir(ctx, eng, cfg, dir); err != nil {
		return err
	}
	if cfg.flushOnExit && cfg.mode != "replay" {
		if err := eng.Flush(ctx); err != nil {
			return fmt.Errorf("flush on exit: %w", err)
		}
	}
	if err := writeMemProfile(cfg.memProfile); err != nil {
		return err
	}
	metrics, err := collectRunMetrics(dir)
	if err != nil {
		return err
	}
	log.Printf(
		"mode=%s field_layout=%s points=%d series=%d query_repeat=%d write_batch_size=%d memtable_max_samples=%d compaction_enabled=%t data_dir=%s",
		cfg.mode,
		normalizedFieldLayout(cfg.fieldLayout),
		cfg.points,
		cfg.series,
		cfg.queryRepeat,
		normalizedWriteBatchSize(cfg),
		normalizedMemTableMaxSamples(cfg),
		cfg.compactionEnabled,
		dir,
	)
	log.Printf(
		"metrics sstable_count=%d data_dir_bytes=%d heap_alloc_bytes=%d heap_sys_bytes=%d heap_total_alloc_bytes=%d mallocs=%d frees=%d num_gc=%d pause_total_ns=%d rss_bytes=%d rss_peak_bytes=%d",
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
	)
	return nil
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("storage_engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&cfg.dataDir, "data-dir", "", "数据目录；为空时使用临时目录并自动清理")
	flags.StringVar(&cfg.mode, "mode", "query", "workload 模式：write/query/compact/replay")
	flags.StringVar(&cfg.fieldLayout, "field-layout", fieldLayoutDefault, "字段布局：default/wide10")
	flags.IntVar(&cfg.points, "points", 10000, "写入点数")
	flags.IntVar(&cfg.series, "series", 100, "series 数量")
	flags.IntVar(&cfg.queryRepeat, "query-repeat", 5, "查询重复次数")
	flags.IntVar(&cfg.writeBatchSize, "write-batch-size", defaultWriteBatchSize, "每次 Engine.Write 提交的 point 数")
	flags.IntVar(&cfg.memTableMaxSamples, "memtable-max-samples", defaultMemTableMaxSamples, "触发 MemTable flush 的 sample 数；wide10 中 1 point=10 samples")
	flags.BoolVar(&cfg.compactionEnabled, "compaction-enabled", false, "启用 size-tiered compaction")
	flags.IntVar(&cfg.compactionLevel0PartLimit, "compaction-level0-part-limit", 0, "Level-0 part 数超过该值时触发 compaction；0 使用引擎默认值")
	flags.Int64Var(&cfg.compactionLevel0SizeLimit, "compaction-level0-size-limit", 0, "Level-0 part 总大小超过该值时触发 compaction；0 表示不按大小触发")
	flags.Int64Var(&cfg.compactionMaxOutputPartBytes, "compaction-max-output-part-bytes", 0, "compaction 输出 part 目标大小；0 表示单 part 输出")
	flags.DurationVar(&cfg.compactionBackgroundInterval, "compaction-background-interval", 0, "后台 compaction 间隔；0 表示不启动后台循环")
	flags.BoolVar(&cfg.flushOnExit, "flush-on-exit", false, "workload 结束后强制 Flush，便于统计完整落盘后的 SSTable 数量")
	flags.StringVar(&cfg.cpuProfile, "cpu-profile", "", "CPU profile 输出文件")
	flags.StringVar(&cfg.memProfile, "mem-profile", "", "heap profile 输出文件")
	flags.BoolVar(&cfg.prebuildPoints, "prebuild-points", false, "在 profile 主阶段前预生成 points")
	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}
	return cfg, nil
}

func validateConfig(cfg config) error {
	switch cfg.mode {
	case "write", "query", "compact", "replay":
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
	if cfg.compactionLevel0PartLimit < 0 {
		return fmt.Errorf("compaction-level0-part-limit must be non-negative")
	}
	if cfg.compactionLevel0SizeLimit < 0 {
		return fmt.Errorf("compaction-level0-size-limit must be non-negative")
	}
	if cfg.compactionMaxOutputPartBytes < 0 {
		return fmt.Errorf("compaction-max-output-part-bytes must be non-negative")
	}
	if cfg.compactionBackgroundInterval < 0 {
		return fmt.Errorf("compaction-background-interval must be non-negative")
	}
	switch normalizedFieldLayout(cfg.fieldLayout) {
	case fieldLayoutDefault, fieldLayoutWide10:
	default:
		return fmt.Errorf("unsupported field layout %q", cfg.fieldLayout)
	}
	return nil
}

func normalizedFieldLayout(layout string) string {
	if layout == "" {
		return fieldLayoutDefault
	}
	return layout
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
			BackgroundInterval: cfg.compactionBackgroundInterval,
		},
	}
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
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
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
			return err
		}
		metrics.dataDirBytes += info.Size()
		return nil
	})
	if err != nil {
		return runMetrics{}, fmt.Errorf("collect run metrics: %w", err)
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

func buildWorkloadPoints(cfg config) []mts.Point {
	points := make([]mts.Point, cfg.points)
	for index := range cfg.points {
		points[index] = workloadPoint(index, cfg.series, cfg.fieldLayout)
	}
	return points
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
	for repeat := range cfg.queryRepeat {
		host := fmt.Sprintf("host-%04d", repeat%cfg.series)
		rows, err := eng.QueryRows(ctx, mts.Query{
			Measurement: "pprof",
			Tags:        map[string]string{"host": host},
			StartTime:   0,
			EndTime:     int64(cfg.points),
		})
		if err != nil {
			return fmt.Errorf("query rows: %w", err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("query host %s returned no rows", host)
		}
	}
	return nil
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
