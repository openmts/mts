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
	storageengine "codeberg.org/mts/mts/internal/engine"
	"codeberg.org/mts/mts/internal/memtable"
	storagemodel "codeberg.org/mts/mts/internal/model"
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
	return errors.Join(
		runCascadeScenario(filepath.Join(dir, "cascade")),
		runReaderCompactionScenario(filepath.Join(dir, "reader")),
		runCorruptCompactionScenario(filepath.Join(dir, "corrupt")),
		runOrphanCleanupScenario(filepath.Join(dir, "orphan")),
		runTombstoneCompactionScenario(filepath.Join(dir, "tombstone")),
	)
}

func runCascadeScenario(dir string) error {
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

func runReaderCompactionScenario(dir string) error {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open reader scenario: %w", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []mts.Point{{
			Measurement: "reader",
			Timestamp:   int64(index + 1),
			Fields:      map[string]mts.FieldValue{"v": mts.Int64Value(int64(index + 1))},
		}}, mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("write reader point: %w", err), closeErr)
		}
	}
	iter, err := eng.QueryColumnIterator(ctx, mts.Query{Measurement: "reader", StartTime: 0, EndTime: 10})
	if err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("open reader iterator: %w", err), closeErr)
	}
	done := make(chan error, 1)
	go func() {
		done <- eng.Compact(ctx)
	}()
	if !iter.Next() {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		return errors.Join(fmt.Errorf("reader iterator returned no data: %w", iter.Err()), closeErr)
	}
	column := iter.Column()
	if len(column.Values) != 2 {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		return errors.Join(fmt.Errorf("reader values = %d, want 2", len(column.Values)), closeErr)
	}
	if err := iter.Close(); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("close reader iterator: %w", err), closeErr)
	}
	select {
	case err := <-done:
		if err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("compact with reader: %w", err), closeErr)
		}
	case <-time.After(time.Second):
		closeErr := eng.Close(ctx)
		return errors.Join(errors.New("compact did not finish after reader close"), closeErr)
	}
	return eng.Close(ctx)
}

func runCorruptCompactionScenario(dir string) error {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open corrupt scenario: %w", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []mts.Point{{
			Measurement: "corrupt",
			Timestamp:   int64(index + 1),
			Fields:      map[string]mts.FieldValue{"v": mts.Int64Value(int64(index + 1))},
		}}, mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			return errors.Join(fmt.Errorf("write corrupt point: %w", err), closeErr)
		}
	}
	shardDir := filepath.Join(dir, "data", "default", "autogen", "shards", "0")
	before, err := sstable.LoadManifest(shardDir)
	if err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("load manifest before corrupt: %w", err), closeErr)
	}
	if len(before.Parts) == 0 {
		closeErr := eng.Close(ctx)
		return errors.Join(errors.New("manifest has no parts before corrupt"), closeErr)
	}
	if err := os.WriteFile(filepath.Join(before.Parts[0].Path, "values.bin"), []byte("bad"), 0600); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("corrupt values: %w", err), closeErr)
	}
	result, compactErr := eng.CompactWithResult(ctx)
	if compactErr == nil || result.State != "failed" {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("compact corrupt error=%v result=%#v, want failed", compactErr, result), closeErr)
	}
	after, err := sstable.LoadManifest(shardDir)
	closeErr := eng.Close(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("load manifest after corrupt: %w", err), closeErr)
	}
	if !sameManifestPartIDs(before, after) {
		return errors.Join(fmt.Errorf("manifest changed after corrupt compact: before=%#v after=%#v", before.Parts, after.Parts), closeErr)
	}
	return closeErr
}

func runOrphanCleanupScenario(dir string) error {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open orphan scenario: %w", err)
	}
	if err := eng.Write(ctx, []mts.Point{{
		Measurement: "orphan",
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"v": mts.Int64Value(1)},
	}}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		return errors.Join(fmt.Errorf("write orphan point: %w", err), closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		return err
	}
	orphanPath := filepath.Join(dir, "data", "default", "autogen", "shards", "0", "sst-orphan")
	if err := os.MkdirAll(orphanPath, 0700); err != nil {
		return fmt.Errorf("create orphan dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(orphanPath, "values.bin"), []byte("orphan"), 0600); err != nil {
		return fmt.Errorf("write orphan file: %w", err)
	}
	reopened, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 1})
	if err != nil {
		return fmt.Errorf("reopen orphan scenario: %w", err)
	}
	closeErr := reopened.Close(ctx)
	if _, err := os.Stat(orphanPath); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(fmt.Errorf("orphan stat = %v, want not exist", err), closeErr)
	}
	return closeErr
}

func sameManifestPartIDs(left sstable.Manifest, right sstable.Manifest) bool {
	if len(left.Parts) != len(right.Parts) {
		return false
	}
	for index := range left.Parts {
		if left.Parts[index].ID != right.Parts[index].ID {
			return false
		}
	}
	return true
}

func runTombstoneCompactionScenario(dir string) error {
	shard, _, err := storageengine.OpenShard(storageengine.ShardOptions{
		Dir:                dir,
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 1,
	})
	if err != nil {
		return fmt.Errorf("open tombstone shard: %w", err)
	}
	points := []storagemodel.ResolvedPoint{
		{
			SeriesID:  1,
			Timestamp: 1,
			WriteSeq:  1,
			Fields: []storagemodel.ResolvedField{
				{FieldID: 1, Type: storagemodel.FieldFloat64, Value: storagemodel.Float64Value(1)},
			},
		},
		{
			SeriesID:  1,
			Timestamp: 2,
			WriteSeq:  2,
			Fields: []storagemodel.ResolvedField{
				{FieldID: 1, Type: storagemodel.FieldFloat64, Value: storagemodel.Float64Value(2)},
			},
		},
	}
	for _, point := range points {
		if err := shard.Write(point, true); err != nil {
			closeErr := shard.Close()
			return errors.Join(fmt.Errorf("write tombstone point: %w", err), closeErr)
		}
	}
	tombstone := storagemodel.Tombstone{
		SeriesIDs: []uint64{1},
		FieldIDs:  []uint32{1},
		StartTime: 1,
		EndTime:   1,
		WriteSeq:  3,
	}
	if err := shard.DeleteRange(tombstone, true); err != nil {
		closeErr := shard.Close()
		return errors.Join(fmt.Errorf("delete tombstone range: %w", err), closeErr)
	}
	if err := shard.Compact(); err != nil {
		closeErr := shard.Close()
		return errors.Join(fmt.Errorf("compact tombstone shard: %w", err), closeErr)
	}
	columns, err := shard.Query(memtable.Query{Start: 0, End: int64(time.Hour)})
	closeErr := shard.Close()
	if err != nil {
		return errors.Join(fmt.Errorf("query tombstone shard: %w", err), closeErr)
	}
	if len(columns) != 1 || len(columns[0].Samples) != 1 || columns[0].Samples[0].Timestamp != 2 {
		return errors.Join(fmt.Errorf("tombstone columns = %#v, want only timestamp 2", columns), closeErr)
	}
	return closeErr
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
