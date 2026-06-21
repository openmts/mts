package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	mts "github.com/openmts/mts"
)

const defaultPoints = 100000

const defaultSeries = 100

const defaultQueryLimit = 2000

type report struct {
	Points                  int     `json:"points"`
	Series                  int     `json:"series"`
	PolicyCount             int     `json:"policy_count"`
	QueryLimit              int     `json:"query_limit"`
	BatchSize               int     `json:"batch_size"`
	CheckpointInterval      int     `json:"checkpoint_interval"`
	RunTimeoutNanos         int64   `json:"run_timeout_nanos"`
	InitialStartUnix        int64   `json:"initial_start_unix"`
	Verify                  bool    `json:"verify"`
	Verified                bool    `json:"verified"`
	WindowsProcessed        int     `json:"windows_processed"`
	PointsWritten           int     `json:"points_written"`
	QueryRows               int     `json:"query_rows"`
	DurationNanos           int64   `json:"duration_nanos"`
	WriteDurationNanos      int64   `json:"write_duration_nanos"`
	DownsampleDurationNanos int64   `json:"downsample_duration_nanos"`
	QueryDurationNanos      int64   `json:"query_duration_nanos"`
	WriteThroughput         float64 `json:"write_throughput_points_per_second"`
	DownsampleThroughput    float64 `json:"downsample_throughput_points_per_second"`
	QueryThroughput         float64 `json:"query_throughput_rows_per_second"`
	RSSPeakBytes            int64   `json:"rss_peak_bytes"`
	WriteRSSPeakBytes       int64   `json:"write_rss_peak_bytes"`
	DownsampleRSSPeakBytes  int64   `json:"downsample_rss_peak_bytes"`
	QueryRSSPeakBytes       int64   `json:"query_rss_peak_bytes"`
	HeapAllocBytes          uint64  `json:"heap_alloc_bytes"`
	HeapInuseBytes          uint64  `json:"heap_inuse_bytes"`
	GCTotal                 uint32  `json:"gc_total"`
	GCPauseTotalNanos       uint64  `json:"gc_pause_total_nanos"`
	DiskBytes               int64   `json:"disk_bytes"`
	SSTableCount            int     `json:"sstable_count"`
	CompletedUntilUnix      int64   `json:"completed_until_unix"`
	StatusCount             int     `json:"status_count"`
}

type config struct {
	points             int
	series             int
	policyCount        int
	queryLimit         int
	batchSize          int
	checkpointInterval int
	runTimeout         time.Duration
	initialStart       int64
	verify             bool
	out                string
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
	dir, err := os.MkdirTemp("", "mts-scale-downsample-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("chmod temp dir: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()
	out, err := runWorkload(context.Background(), dir, cfg)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if cfg.out != "" {
		if err := os.WriteFile(cfg.out, data, 0600); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		return nil
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

func parseConfig(args []string) (config, error) {
	flags := flag.NewFlagSet("downsample_policy", flag.ContinueOnError)
	cfg := config{}
	flags.IntVar(&cfg.points, "points", defaultPoints, "raw point count")
	flags.IntVar(&cfg.series, "series", defaultSeries, "series count")
	flags.IntVar(&cfg.policyCount, "policy-count", 1, "downsample policy count")
	flags.IntVar(&cfg.queryLimit, "query-limit", defaultQueryLimit, "target retention query row limit")
	flags.IntVar(&cfg.batchSize, "batch-size", 1024, "downsample target write batch size")
	flags.IntVar(&cfg.checkpointInterval, "checkpoint-interval", 1, "downsample watermark checkpoint interval")
	flags.DurationVar(&cfg.runTimeout, "run-timeout", 5*time.Minute, "downsample run timeout")
	flags.Int64Var(&cfg.initialStart, "initial-start", 0, "initial downsample start unix nanos")
	flags.BoolVar(&cfg.verify, "verify", true, "verify queried downsample rows")
	flags.StringVar(&cfg.out, "out", "", "report output path")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.points <= 0 {
		return config{}, fmt.Errorf("points must be greater than zero")
	}
	if cfg.series <= 0 {
		return config{}, fmt.Errorf("series must be greater than zero")
	}
	if cfg.series > cfg.points {
		return config{}, fmt.Errorf("series must be less than or equal to points")
	}
	if cfg.policyCount <= 0 {
		return config{}, fmt.Errorf("policy-count must be greater than zero")
	}
	if cfg.queryLimit <= 0 {
		return config{}, fmt.Errorf("query-limit must be greater than zero")
	}
	if cfg.batchSize <= 0 {
		return config{}, fmt.Errorf("batch-size must be greater than zero")
	}
	if cfg.checkpointInterval <= 0 {
		return config{}, fmt.Errorf("checkpoint-interval must be greater than zero")
	}
	if cfg.runTimeout <= 0 {
		return config{}, fmt.Errorf("run-timeout must be greater than zero")
	}
	if cfg.initialStart < 0 {
		return config{}, fmt.Errorf("initial-start must be greater than or equal to zero")
	}
	return cfg, nil
}

func runWorkload(ctx context.Context, dir string, cfg config) (report, error) {
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 8192,
	})
	if err != nil {
		return report{}, fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		_ = eng.Close(ctx)
	}()
	totalStarted := time.Now()
	writeRSS := startRSSMonitor()
	writeStarted := time.Now()
	if err := writeRaw(ctx, eng, cfg); err != nil {
		_ = writeRSS.Stop()
		return report{}, err
	}
	writeDuration := time.Since(writeStarted)
	writeRSSPeak := writeRSS.Stop()
	downsampleRSS := startRSSMonitor()
	downsampleStarted := time.Now()
	result := mts.DownsampleRunResult{}
	for index := 0; index < cfg.policyCount; index++ {
		policy := scalePolicy(cfg, index)
		if err := eng.CreateDownsamplePolicy(ctx, policy); err != nil {
			_ = downsampleRSS.Stop()
			return report{}, fmt.Errorf("create policy: %w", err)
		}
		policyResult, err := eng.RunDownsamplePolicy(ctx, policy.Name, scaleNow(cfg))
		if err != nil {
			_ = downsampleRSS.Stop()
			return report{}, fmt.Errorf("run policy: %w", err)
		}
		result.WindowsProcessed += policyResult.WindowsProcessed
		result.PointsWritten += policyResult.PointsWritten
		if policyResult.CompletedUntilUnix > result.CompletedUntilUnix {
			result.CompletedUntilUnix = policyResult.CompletedUntilUnix
		}
	}
	downsampleDuration := time.Since(downsampleStarted)
	downsampleRSSPeak := downsampleRSS.Stop()
	queryRSS := startRSSMonitor()
	queryStarted := time.Now()
	rows, err := queryTarget(ctx, eng, cfg, result.CompletedUntilUnix)
	if err != nil {
		_ = queryRSS.Stop()
		return report{}, err
	}
	queryDuration := time.Since(queryStarted)
	queryRSSPeak := queryRSS.Stop()
	verified := false
	if cfg.verify {
		if err := verifyRows(rows, cfg); err != nil {
			return report{}, err
		}
		verified = true
	}
	statuses, err := eng.DownsamplePolicyStatuses(ctx, scaleNow(cfg))
	if err != nil {
		return report{}, fmt.Errorf("downsample statuses: %w", err)
	}
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	diskBytes, sstableCount, err := storageFootprint(dir)
	if err != nil {
		return report{}, err
	}
	return report{
		Points:                  cfg.points,
		Series:                  cfg.series,
		PolicyCount:             cfg.policyCount,
		QueryLimit:              cfg.queryLimit,
		BatchSize:               cfg.batchSize,
		CheckpointInterval:      cfg.checkpointInterval,
		RunTimeoutNanos:         cfg.runTimeout.Nanoseconds(),
		InitialStartUnix:        cfg.initialStart,
		Verify:                  cfg.verify,
		Verified:                verified,
		WindowsProcessed:        result.WindowsProcessed,
		PointsWritten:           result.PointsWritten,
		QueryRows:               len(rows),
		DurationNanos:           time.Since(totalStarted).Nanoseconds(),
		WriteDurationNanos:      writeDuration.Nanoseconds(),
		DownsampleDurationNanos: downsampleDuration.Nanoseconds(),
		QueryDurationNanos:      queryDuration.Nanoseconds(),
		WriteThroughput:         throughput(cfg.points, writeDuration),
		DownsampleThroughput:    throughput(result.PointsWritten, downsampleDuration),
		QueryThroughput:         throughput(len(rows), queryDuration),
		RSSPeakBytes:            rssPeakBytes(),
		WriteRSSPeakBytes:       writeRSSPeak,
		DownsampleRSSPeakBytes:  downsampleRSSPeak,
		QueryRSSPeakBytes:       queryRSSPeak,
		HeapAllocBytes:          mem.HeapAlloc,
		HeapInuseBytes:          mem.HeapInuse,
		GCTotal:                 mem.NumGC,
		GCPauseTotalNanos:       mem.PauseTotalNs,
		DiskBytes:               diskBytes,
		SSTableCount:            sstableCount,
		CompletedUntilUnix:      result.CompletedUntilUnix,
		StatusCount:             len(statuses),
	}, nil
}

func storageFootprint(root string) (int64, int, error) {
	var bytes int64
	sstables := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			bytes += info.Size()
			return nil
		}
		if strings.HasPrefix(entry.Name(), "sst-") {
			sstables++
		}
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("scan storage footprint: %w", err)
	}
	return bytes, sstables, nil
}

func writeRaw(ctx context.Context, eng *mts.Engine, cfg config) error {
	const batchSize = 1000
	batch := make([]mts.Point, 0, batchSize)
	for index := 0; index < cfg.points; index++ {
		batch = append(batch, scalePoint(index, cfg.series))
		if len(batch) < batchSize {
			continue
		}
		if err := eng.Write(ctx, batch, mts.WriteOptions{}); err != nil {
			return fmt.Errorf("write batch: %w", err)
		}
		batch = batch[:0]
	}
	if len(batch) == 0 {
		return nil
	}
	if err := eng.Write(ctx, batch, mts.WriteOptions{}); err != nil {
		return fmt.Errorf("write final batch: %w", err)
	}
	return nil
}

func scalePoint(index int, series int) mts.Point {
	host := index % series
	step := index / series
	return mts.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "h" + strconv.Itoa(host)},
		Timestamp:       int64(time.Duration(step) * time.Second),
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(float64(index % 100)),
		},
	}
}

func scalePolicy(cfg config, index int) mts.DownsamplePolicy {
	lookback := time.Duration(cfg.points/cfg.series+120) * time.Second
	name := scalePolicyName(index)
	return mts.DownsamplePolicy{
		Name:              name,
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   scaleTargetRetention(index),
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{
			{Function: "avg", Field: "usage"},
			{Function: "min", Field: "usage"},
			{Function: "max", Field: "usage"},
			{Function: "count", Field: "usage"},
			{Function: "last", Field: "usage"},
		},
		GroupByTags:        []string{"host"},
		RefreshInterval:    time.Minute,
		Lookback:           lookback,
		InitialStartTime:   cfg.initialStart,
		RunTimeout:         cfg.runTimeout,
		BatchSize:          cfg.batchSize,
		CheckpointInterval: cfg.checkpointInterval,
		Enabled:            true,
	}
}

func scalePolicyName(index int) string {
	if index == 0 {
		return "cpu_1m"
	}
	return "cpu_1m_" + strconv.Itoa(index)
}

func scaleTargetRetention(index int) string {
	if index == 0 {
		return "rp_1m"
	}
	return "rp_1m_" + strconv.Itoa(index)
}

func scaleNow(cfg config) time.Time {
	seconds := cfg.points/cfg.series + 120
	return time.Unix(0, int64(time.Duration(seconds)*time.Second))
}

func queryTarget(
	ctx context.Context,
	eng *mts.Engine,
	cfg config,
	end int64,
) ([]mts.Row, error) {
	rows, err := eng.QueryRows(ctx, mts.Query{
		Database:        "metrics",
		RetentionPolicy: scaleTargetRetention(0),
		Measurement:     "cpu",
		Fields: []string{
			"avg_usage",
			"min_usage",
			"max_usage",
			"count_usage",
			"last_usage",
		},
		StartTime: 0,
		EndTime:   end,
		Limit:     cfg.queryLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("query target retention: %w", err)
	}
	return rows, nil
}

func verifyRows(rows []mts.Row, cfg config) error {
	if len(rows) == 0 {
		return fmt.Errorf("query returned no downsample rows")
	}
	for _, row := range rows {
		expected, err := expectedAggregate(row, cfg)
		if err != nil {
			return err
		}
		if err := compareAggregate(row, expected); err != nil {
			return err
		}
	}
	return nil
}

type expectedAggregateValue struct {
	avg   float64
	min   float64
	max   float64
	last  float64
	count int64
}

func expectedAggregate(row mts.Row, cfg config) (expectedAggregateValue, error) {
	host, err := hostIndex(row)
	if err != nil {
		return expectedAggregateValue{}, err
	}
	startSecond := int(row.Timestamp / int64(time.Second))
	endSecond := startSecond + int(time.Minute/time.Second)
	var out expectedAggregateValue
	for step := startSecond; step < endSecond; step++ {
		index := step*cfg.series + host
		if index >= cfg.points {
			break
		}
		value := float64(index % 100)
		if out.count == 0 || value < out.min {
			out.min = value
		}
		if out.count == 0 || value > out.max {
			out.max = value
		}
		out.avg += value
		out.last = value
		out.count++
	}
	if out.count == 0 {
		return expectedAggregateValue{}, fmt.Errorf("row has no expected samples: %#v", row)
	}
	out.avg = out.avg / float64(out.count)
	return out, nil
}

func hostIndex(row mts.Row) (int, error) {
	host := row.Tags["host"]
	if !strings.HasPrefix(host, "h") {
		return 0, fmt.Errorf("row host tag = %q, want hN", host)
	}
	value, err := strconv.Atoi(strings.TrimPrefix(host, "h"))
	if err != nil {
		return 0, fmt.Errorf("parse host tag %q: %w", host, err)
	}
	return value, nil
}

func compareAggregate(row mts.Row, expected expectedAggregateValue) error {
	if got := row.Fields["avg_usage"].Float64; got != expected.avg {
		return fmt.Errorf("avg_usage row=%#v got=%v want=%v", row, got, expected.avg)
	}
	if got := row.Fields["min_usage"].Float64; got != expected.min {
		return fmt.Errorf("min_usage row=%#v got=%v want=%v", row, got, expected.min)
	}
	if got := row.Fields["max_usage"].Float64; got != expected.max {
		return fmt.Errorf("max_usage row=%#v got=%v want=%v", row, got, expected.max)
	}
	if got := row.Fields["last_usage"].Float64; got != expected.last {
		return fmt.Errorf("last_usage row=%#v got=%v want=%v", row, got, expected.last)
	}
	if got := row.Fields["count_usage"].Int64; got != expected.count {
		return fmt.Errorf("count_usage row=%#v got=%v want=%v", row, got, expected.count)
	}
	return nil
}

func throughput(points int, duration time.Duration) float64 {
	if points <= 0 || duration <= 0 {
		return 0
	}
	return float64(points) / duration.Seconds()
}

type rssMonitor struct {
	done    chan struct{}
	stopped chan struct{}
	peak    atomic.Int64
}

func startRSSMonitor() *rssMonitor {
	monitor := &rssMonitor{
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	monitor.observe()
	go monitor.loop()
	return monitor
}

func (m *rssMonitor) loop() {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	defer close(m.stopped)
	for {
		select {
		case <-ticker.C:
			m.observe()
		case <-m.done:
			m.observe()
			return
		}
	}
}

func (m *rssMonitor) Stop() int64 {
	close(m.done)
	<-m.stopped
	return m.peak.Load()
}

func (m *rssMonitor) observe() {
	value := currentRSSBytes()
	for {
		previous := m.peak.Load()
		if value <= previous || m.peak.CompareAndSwap(previous, value) {
			return
		}
	}
}

func currentRSSBytes() int64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kib, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kib * 1024
	}
	return 0
}

func rssPeakBytes() int64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kib, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kib * 1024
	}
	return 0
}
