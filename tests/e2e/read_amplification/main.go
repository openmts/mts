package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mts "github.com/openmts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("read_amplification failed: %v", err)
	}
	log.Print("read_amplification passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-read-amp-*")
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

func runWithDir(dir string) (err error) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	for index := range 4 {
		if err := eng.Write(ctx, []mts.Point{readAmpPoint(int64(index))}, mts.WriteOptions{}); err != nil {
			return fmt.Errorf("write point %d: %w", index, err)
		}
	}
	_, err = eng.QueryColumns(ctx, mts.Query{
		Measurement: "read_amp",
		StartTime:   0,
		EndTime:     10,
		Budget:      mts.QueryBudget{MaxParts: 1},
	})
	if !errors.Is(err, mts.ErrReadBudgetExceeded) {
		return fmt.Errorf("parts budget error = %v, want read budget exceeded", err)
	}
	columns, err := eng.QueryColumns(ctx, mts.Query{
		Measurement: "read_amp",
		StartTime:   0,
		EndTime:     10,
		Budget:      mts.QueryBudget{MaxSamples: 1},
	})
	if !errors.Is(err, mts.ErrReadBudgetExceeded) {
		stats := eng.QueryStatsSnapshot()
		return fmt.Errorf("sample budget error = %v columns=%#v stats=%#v, want read budget exceeded", err, columns, stats)
	}
	if stats := eng.QueryStatsSnapshot(); stats.Errors == 0 {
		return fmt.Errorf("query stats = %#v, want recorded budget error", stats)
	}
	return nil
}

func readAmpPoint(timestamp int64) mts.Point {
	return mts.Point{
		Measurement: "read_amp",
		Timestamp:   timestamp,
		Fields:      map[string]mts.FieldValue{"value": mts.Int64Value(timestamp)},
	}
}
