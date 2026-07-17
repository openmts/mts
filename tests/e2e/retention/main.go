package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	mts "github.com/openmts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("retention failed: %v", err)
	}
	log.Print("retention passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-retention-*")
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
	return runWithDir(dir)
}

func runWithDir(dir string) error {
	if err := runWholeShardRetention(dir); err != nil {
		return err
	}
	return runPartialShardRetention(filepath.Join(dir, "partial"))
}

func runWholeShardRetention(dir string) error {
	ctx := context.Background()
	opts := mts.Options{Path: dir, ShardDuration: time.Hour, Retention: time.Hour, MemTableMaxSamples: 10}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	if err := eng.Write(ctx, []mts.Point{point(0, 1), point(int64(2*time.Hour), 2)}, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write points: %w", err), closeErr)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("apply retention: %w", err), closeErr)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "retention", StartTime: 0, EndTime: int64(3 * time.Hour)})
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return assertRetainedRows(rows)
}

// runPartialShardRetention 验证活跃分片内过期点会被 tombstone 删除。
func runPartialShardRetention(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir partial dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("chmod partial dir: %w", err)
	}
	ctx := context.Background()
	opts := mts.Options{Path: dir, ShardDuration: time.Hour, Retention: 30 * time.Minute, MemTableMaxSamples: 10}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("open partial engine: %w", err)
	}
	if err := eng.Write(ctx, []mts.Point{
		point(int64(10*time.Minute), 1),
		point(int64(50*time.Minute), 2),
	}, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write partial points: %w", err), closeErr)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(70*time.Minute))); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("apply partial retention: %w", err), closeErr)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "retention", StartTime: 0, EndTime: int64(time.Hour)})
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query partial rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return assertRetainedRows(rows)
}

func point(timestamp int64, value float64) mts.Point {
	return mts.Point{
		Measurement: "retention",
		Timestamp:   timestamp,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(value)},
	}
}

func assertRetainedRows(rows []mts.Row) error {
	if len(rows) != 1 || rows[0].Fields["v"].Float64 != 2 {
		return fmt.Errorf("rows = %#v, want only retained new shard", rows)
	}
	return nil
}
