package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	mts "codeberg.org/mts/mts"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("flush_manifest_recovery failed: %v", err)
	}
	log.Print("flush_manifest_recovery passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-flush-manifest-*")
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
	opts := mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 10}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	if err := eng.Write(ctx, []mts.Point{point(10, 1)}, mts.WriteOptions{Sync: true}); err != nil {
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
	if _, err := sstable.WritePart(shardDir(dir), 0, "sst-orphan", orphanColumn()); err != nil {
		return fmt.Errorf("write orphan part: %w", err)
	}

	reopened, err := mts.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("reopen engine: %w", err)
	}
	rows, err := reopened.QueryRows(ctx, mts.Query{Measurement: "flush", StartTime: 0, EndTime: 20})
	closeErr := reopened.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(rows) != 1 || rows[0].Fields["v"].Float64 != 1 {
		return fmt.Errorf("rows = %#v, want only manifest referenced row", rows)
	}
	return nil
}

func point(timestamp int64, value float64) mts.Point {
	return mts.Point{
		Measurement: "flush",
		Timestamp:   timestamp,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(value)},
	}
}

func shardDir(root string) string {
	return filepath.Join(root, "data", "default", "autogen", "shards", "0")
}

func orphanColumn() []model.ColumnData {
	return []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 15, WriteSeq: 99, Value: model.Float64Value(99)},
		},
	}}
}
