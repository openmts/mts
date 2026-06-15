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
		log.Fatalf("compaction_integrity failed: %v", err)
	}
	log.Print("compaction_integrity passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-compaction-*")
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
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	for value := 1; value <= 3; value++ {
		if err := eng.Write(ctx, []mts.Point{point(float64(value))}, mts.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("write value %d: %w", value, err), closeErr)
		}
	}
	if err := eng.Compact(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("compact: %w", err), closeErr)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "compact", StartTime: 0, EndTime: 20})
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(rows) != 1 || rows[0].Fields["v"].Float64 != 3 {
		return fmt.Errorf("rows = %#v, want compacted LWW value 3", rows)
	}
	return nil
}

func point(value float64) mts.Point {
	return mts.Point{
		Measurement: "compact",
		Timestamp:   10,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(value)},
	}
}
