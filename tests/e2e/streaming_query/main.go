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
		log.Fatalf("streaming_query failed: %v", err)
	}
	log.Print("streaming_query passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-streaming-*")
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
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 256})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	if err := eng.Write(ctx, streamingPoints(2000), mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write points: %w", err)
	}
	if err := eng.Flush(ctx); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	iter, err := eng.QueryColumnIterator(ctx, mts.Query{Measurement: "streaming", StartTime: 0, EndTime: 1999})
	if err != nil {
		return fmt.Errorf("query iterator: %w", err)
	}
	defer func() {
		err = errors.Join(err, iter.Close())
	}()
	columns := 0
	values := 0
	for iter.Next() {
		column := iter.Column()
		columns++
		values += len(column.Values)
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("iterator err: %w", err)
	}
	if columns != 1 || values != 2000 {
		return fmt.Errorf("streamed columns=%d values=%d, want 1 and 2000", columns, values)
	}
	return nil
}

func streamingPoints(count int) []mts.Point {
	points := make([]mts.Point, 0, count)
	for index := range count {
		points = append(points, mts.Point{
			Measurement: "streaming",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(index),
			Fields:      map[string]mts.FieldValue{"value": mts.Float64Value(float64(index))},
		})
	}
	return points
}
