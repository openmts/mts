package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	mts "codeberg.org/mts/mts"
	"codeberg.org/mts/mts/internal/sstable"
)

type soakReport struct {
	Seed              int64         `json:"seed"`
	Duration          time.Duration `json:"duration"`
	Iterations        int           `json:"iterations"`
	Rows              int           `json:"rows"`
	OK                bool          `json:"ok"`
	PartCount         int           `json:"part_count"`
	LevelDistribution map[int]int   `json:"level_distribution"`
	HealthDegraded    bool          `json:"health_degraded"`
	CompactionBacklog int           `json:"compaction_backlog"`
	Writes            int           `json:"writes"`
	Queries           int           `json:"queries"`
	Compactions       int           `json:"compactions"`
	Restarts          int           `json:"restarts"`
	Recoveries        int           `json:"recoveries"`
}

type periodicSoakReport struct {
	UnixNano          int64 `json:"unix_nano"`
	Iteration         int   `json:"iteration"`
	Rows              int   `json:"rows"`
	Writes            int   `json:"writes"`
	Queries           int   `json:"queries"`
	Compactions       int   `json:"compactions"`
	Restarts          int   `json:"restarts"`
	Recoveries        int   `json:"recoveries"`
	PartCount         int   `json:"part_count"`
	CompactionBacklog int   `json:"compaction_backlog"`
}

type soakConfig struct {
	seed           int64
	duration       time.Duration
	reportInterval time.Duration
	reportPath     string
	dataDir        string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (err error) {
	cfg, err := parseSoakConfig(args)
	if err != nil {
		return err
	}
	dir, cleanup, err := prepareSoakDir(cfg.dataDir)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()
	writer, closeWriter, err := openSoakReportWriter(cfg.reportPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeWriter())
	}()
	report, err := runSoakWithReports(dir, cfg, writer)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func runSoak(dir string, seed int64, duration time.Duration) (soakReport, error) {
	return runSoakWithReports(dir, soakConfig{seed: seed, duration: duration}, io.Discard)
}

func parseSoakConfig(args []string) (soakConfig, error) {
	flags := flag.NewFlagSet("storage_soak", flag.ContinueOnError)
	seed := flags.Int64("seed", 1, "seed")
	duration := flags.Duration("duration", time.Second, "duration")
	reportInterval := flags.Duration("report-interval", time.Second, "periodic report interval")
	reportPath := flags.String("report-jsonl", "", "periodic report jsonl path")
	dataDir := flags.String("data-dir", "", "data directory")
	if err := flags.Parse(args); err != nil {
		return soakConfig{}, err
	}
	if *duration <= 0 || *reportInterval < 0 {
		return soakConfig{}, fmt.Errorf("duration must be positive and report interval must be non-negative")
	}
	return soakConfig{
		seed:           *seed,
		duration:       *duration,
		reportInterval: *reportInterval,
		reportPath:     *reportPath,
		dataDir:        *dataDir,
	}, nil
}

func prepareSoakDir(path string) (string, func() error, error) {
	if path != "" {
		if err := os.MkdirAll(path, 0700); err != nil {
			return "", nil, fmt.Errorf("create data dir: %w", err)
		}
		return path, func() error { return nil }, nil
	}
	dir, err := os.MkdirTemp("", "mts-soak-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("chmod temp dir: %w", err)
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func openSoakReportWriter(path string) (io.Writer, func() error, error) {
	if path == "" {
		return io.Discard, func() error { return nil }, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("open report jsonl: %w", err)
	}
	return file, file.Close, nil
}

func runSoakWithReports(dir string, cfg soakConfig, writer io.Writer) (soakReport, error) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 64,
		Compaction:         mts.CompactionOptions{Enabled: true, Level0PartLimit: 2, MaxCascadeSteps: 4},
	})
	if err != nil {
		return soakReport{}, fmt.Errorf("open engine: %w", err)
	}
	random := rand.New(rand.NewSource(cfg.seed))
	deadline := time.Now().Add(cfg.duration)
	nextReport := time.Now().Add(cfg.reportInterval)
	iterations := 0
	expected := make(map[int64]int64)
	counts := soakCounts{}
	for iterations == 0 || time.Now().Before(deadline) {
		points := soakPoints(iterations, random.Intn(8)+1)
		for _, point := range points {
			expected[point.Timestamp] = point.Fields["value"].Int64
		}
		if err := eng.Write(ctx, points, mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(fmt.Errorf("write points: %w", err), closeErr)
		}
		counts.writes++
		if iterations%3 == 0 {
			if err := eng.Flush(ctx); err != nil {
				closeErr := eng.Close(ctx)
				return soakReport{}, errors.Join(fmt.Errorf("flush: %w", err), closeErr)
			}
		}
		if iterations%5 == 0 {
			if err := eng.Compact(ctx); err != nil {
				closeErr := eng.Close(ctx)
				return soakReport{}, errors.Join(fmt.Errorf("compact: %w", err), closeErr)
			}
			counts.compactions++
		}
		queryEnd := int64((iterations + 1) * 1000)
		rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "soak", StartTime: 0, EndTime: queryEnd})
		if err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
		}
		if err := verifyRows(rows, expected, cfg.seed); err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(err, closeErr)
		}
		counts.queries++
		iterations++
		if iterations%7 == 0 {
			next, err := reopenSoakEngine(ctx, eng, dir)
			if err != nil {
				return soakReport{}, err
			}
			eng = next
			counts.restarts++
			counts.recoveries++
		}
		if shouldWriteSoakReport(cfg.reportInterval, nextReport) {
			if err := writePeriodicSoakReport(writer, dir, eng, iterations, len(expected), counts); err != nil {
				closeErr := eng.Close(ctx)
				return soakReport{}, errors.Join(err, closeErr)
			}
			nextReport = time.Now().Add(cfg.reportInterval)
		}
	}
	if counts.restarts == 0 {
		next, err := reopenSoakEngine(ctx, eng, dir)
		if err != nil {
			return soakReport{}, err
		}
		eng = next
		counts.restarts++
		counts.recoveries++
	}
	stats := eng.CompactionStatsSnapshot()
	health := eng.HealthSnapshot()
	if err := eng.Close(ctx); err != nil {
		return soakReport{}, err
	}
	partCount, err := countSoakSSTables(dir)
	if err != nil {
		return soakReport{}, err
	}
	levels, err := soakLevelDistribution(dir)
	if err != nil {
		return soakReport{}, err
	}
	return soakReport{
		Seed:              cfg.seed,
		Duration:          cfg.duration,
		Iterations:        iterations,
		Rows:              len(expected),
		OK:                true,
		PartCount:         partCount,
		LevelDistribution: levels,
		HealthDegraded:    !health.Healthy || !health.Ready,
		CompactionBacklog: stats.Backlog,
		Writes:            counts.writes,
		Queries:           counts.queries,
		Compactions:       counts.compactions,
		Restarts:          counts.restarts,
		Recoveries:        counts.recoveries,
	}, nil
}

type soakCounts struct {
	writes      int
	queries     int
	compactions int
	restarts    int
	recoveries  int
}

func reopenSoakEngine(ctx context.Context, eng *mts.Engine, dir string) (*mts.Engine, error) {
	if err := eng.Close(ctx); err != nil {
		return nil, err
	}
	next, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 64,
		Compaction:         mts.CompactionOptions{Enabled: true, Level0PartLimit: 2, MaxCascadeSteps: 4},
	})
	if err != nil {
		return nil, fmt.Errorf("reopen engine: %w", err)
	}
	return next, nil
}

func shouldWriteSoakReport(interval time.Duration, next time.Time) bool {
	if interval <= 0 {
		return false
	}
	return !time.Now().Before(next)
}

func writePeriodicSoakReport(
	writer io.Writer,
	dir string,
	eng *mts.Engine,
	iteration int,
	rows int,
	counts soakCounts,
) error {
	stats := eng.CompactionStatsSnapshot()
	partCount, err := countSoakSSTables(dir)
	if err != nil {
		return err
	}
	record := periodicSoakReport{
		UnixNano:          time.Now().UnixNano(),
		Iteration:         iteration,
		Rows:              rows,
		Writes:            counts.writes,
		Queries:           counts.queries,
		Compactions:       counts.compactions,
		Restarts:          counts.restarts,
		Recoveries:        counts.recoveries,
		PartCount:         partCount,
		CompactionBacklog: stats.Backlog,
	}
	return json.NewEncoder(writer).Encode(record)
}

func verifyRows(rows []mts.Row, expected map[int64]int64, seed int64) error {
	if len(rows) != len(expected) {
		return fmt.Errorf("rows=%d want=%d seed=%d", len(rows), len(expected), seed)
	}
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		want, ok := expected[row.Timestamp]
		if !ok {
			return fmt.Errorf("unexpected timestamp=%d seed=%d", row.Timestamp, seed)
		}
		if _, duplicate := seen[row.Timestamp]; duplicate {
			return fmt.Errorf("duplicate timestamp=%d seed=%d", row.Timestamp, seed)
		}
		seen[row.Timestamp] = struct{}{}
		value, ok := row.Fields["value"]
		if !ok {
			return fmt.Errorf("missing value field timestamp=%d seed=%d", row.Timestamp, seed)
		}
		if value.Type != mts.FieldInt64 || value.Int64 != want {
			return fmt.Errorf("value timestamp=%d got=%+v want=%d seed=%d", row.Timestamp, value, want, seed)
		}
	}
	return nil
}

func soakPoints(iteration int, count int) []mts.Point {
	points := make([]mts.Point, 0, count)
	base := iteration * 1000
	for index := range count {
		timestamp := int64(base + index)
		points = append(points, mts.Point{
			Measurement: "soak",
			Tags:        map[string]string{"host": fmt.Sprintf("host-%02d", iteration%4)},
			Timestamp:   timestamp,
			Fields:      map[string]mts.FieldValue{"value": mts.Int64Value(timestamp)},
		})
	}
	return points
}

func countSoakSSTables(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.IsDir() {
			return err
		}
		if len(info.Name()) >= 4 && info.Name()[:4] == "sst-" {
			count++
		}
		_ = path
		return nil
	})
	return count, err
}

func soakLevelDistribution(root string) (map[int]int, error) {
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
