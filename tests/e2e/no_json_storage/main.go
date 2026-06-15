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
		log.Fatalf("no_json_storage failed: %v", err)
	}
	log.Print("no_json_storage passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-no-json-*")
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
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	point := mts.Point{
		Measurement: "bin",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"value": mts.Float64Value(1),
			"state": mts.StringValue("ok"),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write point: %w", err), closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("flush: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close engine: %w", err)
	}
	return assertNoJSONStorage(dir)
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
