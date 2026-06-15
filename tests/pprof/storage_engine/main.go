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
	"time"

	mts "codeberg.org/mts/mts"
)

type config struct {
	dataDir     string
	mode        string
	points      int
	series      int
	queryRepeat int
	cpuProfile  string
	memProfile  string
}

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
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 8192})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()

	if err := runWorkloadWithDir(ctx, eng, cfg, dir); err != nil {
		return err
	}
	if err := writeMemProfile(cfg.memProfile); err != nil {
		return err
	}
	log.Printf("mode=%s points=%d series=%d query_repeat=%d data_dir=%s", cfg.mode, cfg.points, cfg.series, cfg.queryRepeat, dir)
	return nil
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("storage_engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&cfg.dataDir, "data-dir", "", "数据目录；为空时使用临时目录并自动清理")
	flags.StringVar(&cfg.mode, "mode", "query", "workload 模式：write/query/compact/replay")
	flags.IntVar(&cfg.points, "points", 10000, "写入点数")
	flags.IntVar(&cfg.series, "series", 100, "series 数量")
	flags.IntVar(&cfg.queryRepeat, "query-repeat", 5, "查询重复次数")
	flags.StringVar(&cfg.cpuProfile, "cpu-profile", "", "CPU profile 输出文件")
	flags.StringVar(&cfg.memProfile, "mem-profile", "", "heap profile 输出文件")
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
	return nil
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
		if err := eng.Write(ctx, []mts.Point{workloadPoint(index, cfg.series)}, mts.WriteOptions{}); err != nil {
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
	reopened, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 8192})
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
	const batchSize = 1024
	batch := make([]mts.Point, 0, batchSize)
	for index := range cfg.points {
		batch = append(batch, workloadPoint(index, cfg.series))
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
	const batchSize = 1024
	batch := make([]mts.Point, 0, batchSize)
	for index := range cfg.points {
		batch = append(batch, workloadPoint(index, cfg.series))
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

func workloadPoint(index int, series int) mts.Point {
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
