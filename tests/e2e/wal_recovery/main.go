package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mts "codeberg.org/mts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("wal_recovery failed: %v", err)
	}
	log.Print("wal_recovery passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-wal-recovery-*")
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

	ctx := context.Background()
	opts := mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 2000}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	if err := eng.Write(ctx, points(1000), mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write points: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close engine: %w", err)
	}

	reopened, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("reopen engine: %w", err)
	}
	rows, err := reopened.QueryRows(ctx, mts.Query{Measurement: "wal", StartTime: 0, EndTime: 999})
	closeErr := reopened.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(rows) != 1000 {
		return fmt.Errorf("row count = %d, want 1000", len(rows))
	}
	return nil
}

func points(count int) []mts.Point {
	out := make([]mts.Point, 0, count)
	for index := range count {
		out = append(out, mts.Point{
			Measurement: "wal",
			Timestamp:   int64(index),
			Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(float64(index))},
		})
	}
	return out
}
