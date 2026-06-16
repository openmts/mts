package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	mts "codeberg.org/mts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("query_pruning failed: %v", err)
	}
	log.Print("query_pruning passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-query-pruning-*")
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
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1000})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	if err := eng.Write(ctx, points(), mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write points: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush: %w", err), closeErr)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "prune",
		Tags:        map[string]string{"host": "host-042"},
		Fields:      []string{"f2"},
		StartTime:   0,
		EndTime:     1000,
	})
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := assertPrunedRows(rows); err != nil {
		return err
	}
	return assertNoJSONStorage(dir)
}

func assertPrunedRows(rows []mts.Row) error {
	if len(rows) != 1 || len(rows[0].Fields) != 1 || rows[0].Fields["f2"].Int64 != 42 {
		return fmt.Errorf("rows = %#v, want one pruned field result", rows)
	}
	return nil
}

func points() []mts.Point {
	out := make([]mts.Point, 0, 100)
	for index := range 100 {
		out = append(out, mts.Point{
			Measurement: "prune",
			Tags:        map[string]string{"host": fmt.Sprintf("host-%03d", index)},
			Timestamp:   int64(index),
			Fields: map[string]mts.FieldValue{
				"f1": mts.Float64Value(float64(index)),
				"f2": mts.Int64Value(int64(index)),
				"f3": mts.StringValue("ok"),
				"f4": mts.BoolValue(index%2 == 0),
			},
		})
	}
	return out
}

func assertNoJSONStorage(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if bytes.Contains(data, []byte(`{"`)) || bytes.Contains(data, []byte(`":`)) {
			return fmt.Errorf("file %s appears to contain JSON payload", path)
		}
		return nil
	})
}
