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
	"codeberg.org/mts/mts/internal/sstable"
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
	return runWithDir(dir)
}

func runWithDir(dir string) error {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
		Compaction: mts.CompactionOptions{
			Enabled:         true,
			MaxCascadeSteps: 4,
			Levels: []mts.CompactionLevelOptions{
				{Level: 0, PartLimit: 1},
				{Level: 1, PartLimit: 1},
				{Level: 2, PartLimit: 4},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	for value := 1; value <= 4; value++ {
		if err := eng.Write(ctx, []mts.Point{point(float64(value))}, mts.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("write value %d: %w", value, err), closeErr)
		}
	}
	if err := eng.Compact(ctx); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("compact: %w", err), closeErr)
	}
	if err := assertLevelTwoPart(dir); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(err, closeErr)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "compact", StartTime: 0, EndTime: 20})
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return assertCompactedRows(rows)
}

func assertLevelTwoPart(root string) error {
	manifest, err := sstable.LoadManifest(filepath.Join(root, "data", "default", "autogen", "shards", "0"))
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	for _, part := range manifest.Parts {
		if part.Level >= 2 {
			return nil
		}
	}
	return fmt.Errorf("manifest parts = %#v, want at least one L2 part", manifest.Parts)
}

func point(value float64) mts.Point {
	return mts.Point{
		Measurement: "compact",
		Timestamp:   10,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(value)},
	}
}

func assertCompactedRows(rows []mts.Row) error {
	if len(rows) != 1 || rows[0].Fields["v"].Float64 != 4 {
		return fmt.Errorf("rows = %#v, want compacted LWW value 4", rows)
	}
	return nil
}
