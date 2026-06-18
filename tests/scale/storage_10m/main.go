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
	"syscall"
	"time"

	mts "codeberg.org/mts/mts"
	"codeberg.org/mts/mts/internal/sstable"
)

type report struct {
	Profile            string              `json:"profile"`
	Mode               string              `json:"mode"`
	Points             int                 `json:"points"`
	Duration           time.Duration       `json:"duration"`
	Throughput         float64             `json:"throughput"`
	HeapAlloc          uint64              `json:"heap_alloc"`
	HeapSys            uint64              `json:"heap_sys"`
	TotalAlloc         uint64              `json:"total_alloc"`
	Mallocs            uint64              `json:"mallocs"`
	Frees              uint64              `json:"frees"`
	NumGC              uint32              `json:"num_gc"`
	RSSPeakBytes       int64               `json:"rss_peak_bytes"`
	Rows               int                 `json:"rows"`
	DataBytes          int64               `json:"data_bytes"`
	SSTableCount       int                 `json:"sstable_count"`
	LevelDistribution  map[int]int         `json:"level_distribution"`
	ReadAmplification  float64             `json:"read_amplification"`
	WriteAmplification float64             `json:"write_amplification"`
	SpaceAmplification float64             `json:"space_amplification"`
	QueryLatencyNanos  int64               `json:"query_latency_nanos"`
	ColdQueryLatency   int64               `json:"cold_query_latency_nanos"`
	HotQueryLatency    int64               `json:"hot_query_latency_nanos"`
	BacklogDrainNanos  int64               `json:"backlog_drain_nanos"`
	CompactionStats    mts.CompactionStats `json:"compaction_stats"`
	Errors             []string            `json:"errors"`
}

type config struct {
	profile              string
	mode                 string
	points               int
	batchSize            int
	dataDir              string
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
	tableCount, err := countSSTables(dir)
	if err != nil {
		return err
	}
	levelDistribution, err := levelDistribution(dir)
	if err != nil {
		return err
	}
	logicalBytes := logicalInputBytes(cfg.points)
	out := report{
		Profile:            cfg.profile,
		Mode:               cfg.mode,
		Points:             cfg.points,
		Duration:           duration,
		Throughput:         float64(cfg.points) / duration.Seconds(),
		HeapAlloc:          mem.HeapAlloc,
		HeapSys:            mem.HeapSys,
		TotalAlloc:         mem.TotalAlloc,
		Mallocs:            mem.Mallocs,
		Frees:              mem.Frees,
		NumGC:              mem.NumGC,
		RSSPeakBytes:       rssPeakBytes(),
		Rows:               workload.rows,
		DataBytes:          dataBytes,
		SSTableCount:       tableCount,
		LevelDistribution:  levelDistribution,
		ReadAmplification:  readAmplification(tableCount, workload.rows),
		WriteAmplification: amplificationRatio(dataBytes, logicalBytes),
		SpaceAmplification: amplificationRatio(dataBytes, logicalBytes),
		QueryLatencyNanos:  workload.queryLatency.Nanoseconds(),
		ColdQueryLatency:   workload.coldQueryLatency.Nanoseconds(),
		HotQueryLatency:    workload.hotQueryLatency.Nanoseconds(),
		BacklogDrainNanos:  workload.backlogDrain.Nanoseconds(),
		CompactionStats:    workload.compactionStats,
		Errors:             []string{},
	}
	if err := compareBaseline(cfg, out); err != nil {
		return err
	}
	if err := enforceThresholds(cfg, out); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func parseConfig(args []string) (config, error) {
	flags := flag.NewFlagSet("storage_10m", flag.ContinueOnError)
	profile := flags.String("profile", "quick", "profile: quick|standard|soak")
	mode := flags.String("mode", "write", "mode")
	points := flags.Int("points", 0, "points")
	batchSize := flags.Int("batch-size", 1024, "batch size")
	dataDir := flags.String("data-dir", "", "data directory")
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
	if *points <= 0 || *batchSize <= 0 {
		return config{}, fmt.Errorf("points and batch-size must be positive")
	}
	if !validProfile(*profile) {
		return config{}, fmt.Errorf("unsupported profile %q", *profile)
	}
	if *maxRSSBytes < 0 || *maxSSTables < 0 || *maxBacklog < 0 {
		return config{}, fmt.Errorf("thresholds must be non-negative")
	}
	switch *mode {
	case "write", "query", "compact", "restart":
	default:
		return config{}, fmt.Errorf("unsupported mode %q", *mode)
	}
	return config{
		profile:              *profile,
		mode:                 *mode,
		points:               *points,
		batchSize:            *batchSize,
		dataDir:              *dataDir,
		baseline:             *baseline,
		maxRegressionPercent: *maxRegression,
		maxRSSBytes:          *maxRSSBytes,
		maxSSTableCount:      *maxSSTables,
		maxCompactionBacklog: *maxBacklog,
	}, nil
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
	rows             int
	queryLatency     time.Duration
	coldQueryLatency time.Duration
	hotQueryLatency  time.Duration
	backlogDrain     time.Duration
	compactionStats  mts.CompactionStats
}

func runWorkloadDetailed(dir string, cfg config) (workloadResult, error) {
	ctx := context.Background()
	result := workloadResult{}
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 8192})
	if err != nil {
		return workloadResult{}, fmt.Errorf("open engine: %w", err)
	}
	for start := 0; start < cfg.points; start += cfg.batchSize {
		end := start + cfg.batchSize
		if end > cfg.points {
			end = cfg.points
		}
		if err := eng.Write(ctx, scalePoints(start, end), mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			return workloadResult{}, errors.Join(fmt.Errorf("write batch: %w", err), closeErr)
		}
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return workloadResult{}, errors.Join(fmt.Errorf("flush: %w", err), closeErr)
	}
	if cfg.mode == "compact" {
		compactStarted := time.Now()
		if err := eng.Compact(ctx); err != nil {
			closeErr := eng.Close(ctx)
			return workloadResult{}, errors.Join(fmt.Errorf("compact: %w", err), closeErr)
		}
		result.backlogDrain = time.Since(compactStarted)
	}
	rows := 0
	if cfg.mode == "query" || cfg.mode == "compact" || cfg.mode == "restart" {
		got, latency, err := timedQueryRows(ctx, eng, cfg.points)
		if err != nil {
			closeErr := eng.Close(ctx)
			return workloadResult{}, errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
		}
		_, hotLatency, err := timedQueryRows(ctx, eng, cfg.points)
		if err != nil {
			closeErr := eng.Close(ctx)
			return workloadResult{}, errors.Join(fmt.Errorf("query rows hot: %w", err), closeErr)
		}
		rows = len(got)
		result.queryLatency = latency
		result.coldQueryLatency = latency
		result.hotQueryLatency = hotLatency
	}
	stats := eng.CompactionStatsSnapshot()
	result.rows = rows
	result.compactionStats = stats
	if err := eng.Close(ctx); err != nil {
		return workloadResult{}, err
	}
	if cfg.mode != "restart" {
		return result, nil
	}
	reopened, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 8192})
	if err != nil {
		return workloadResult{}, fmt.Errorf("reopen engine: %w", err)
	}
	got, latency, err := timedQueryRows(ctx, reopened, cfg.points)
	closeErr := reopened.Close(ctx)
	if err != nil {
		return workloadResult{}, errors.Join(fmt.Errorf("query reopened: %w", err), closeErr)
	}
	result.rows = len(got)
	result.queryLatency = latency
	return result, closeErr
}

func timedQueryRows(ctx context.Context, eng *mts.Engine, points int) ([]mts.Row, time.Duration, error) {
	started := time.Now()
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "scale", StartTime: 0, EndTime: int64(points)})
	return rows, time.Since(started), err
}

func scalePoints(start int, end int) []mts.Point {
	points := make([]mts.Point, 0, end-start)
	for index := start; index < end; index++ {
		points = append(points, mts.Point{
			Measurement: "scale",
			Tags:        map[string]string{"host": fmt.Sprintf("host-%03d", index%100)},
			Timestamp:   int64(index),
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

func amplificationRatio(actual int64, logical int64) float64 {
	if actual <= 0 || logical <= 0 {
		return 0
	}
	return float64(actual) / float64(logical)
}

func rssPeakBytes() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return usage.Maxrss * 1024
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
