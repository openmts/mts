package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/mts/mts/internal/catalog"
	"codeberg.org/mts/mts/internal/memtable"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
	"codeberg.org/mts/mts/internal/wal"
)

func TestEngineLifecycleAndQueries(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		Retention:          time.Hour,
		MemTableMaxSamples: 2,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   10,
		Fields: map[string]model.FieldValue{
			"state": model.StringValue("ok"),
			"usage": model.Float64Value(1),
		},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryColumns() error = %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("column count = %d, want 2", len(columns))
	}
	iter, err := eng.QueryColumnIterator(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryColumnIterator() error = %v", err)
	}
	iterCount := 0
	for iter.Next() {
		if iter.Column().SeriesID == 0 {
			t.Fatal("iterator returned empty column")
		}
		iterCount++
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("iterator Err() = %v", err)
	}
	if err := iter.Close(); err != nil {
		t.Fatalf("iterator Close() error = %v", err)
	}
	if iterCount != 2 {
		t.Fatalf("iterator column count = %d, want 2", iterCount)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "cpu", StartTime: 0, EndTime: 20})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if err := eng.Compact(ctx); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	rows, err = eng.QueryRows(ctx, model.Query{Measurement: "cpu", StartTime: 0, EndTime: 20})
	if err != nil {
		t.Fatalf("QueryRows() after retention error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("row count after retention = %d, want 0", len(rows))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQuerySnapshotAllowsConcurrentWrite(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 200)
	for index := range 200 {
		points = append(points, model.Point{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(index),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(index))},
		})
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write(seed) error = %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, queryErr := eng.QueryRows(ctx, model.Query{
			Measurement: "cpu",
			StartTime:   0,
			EndTime:     199,
		})
		done <- queryErr
	}()
	<-started
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- eng.Write(ctx, []model.Point{{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   201,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(201)},
		}}, model.WriteOptions{})
	}()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("concurrent Write() error = %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("concurrent Write() blocked behind query")
	}
	if err := <-done; err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestEngineReopenAndNoMatches(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "mem",
		Timestamp:   15,
		Fields:      map[string]model.FieldValue{"used": model.Int64Value(9)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eng, err = Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "missing", StartTime: 0, EndTime: 20})
	if err != nil {
		t.Fatalf("QueryRows() no matches error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("missing rows = %d, want 0", len(rows))
	}
	rows, err = eng.QueryRows(ctx, model.Query{Measurement: "mem", Fields: []string{"missing"}, StartTime: 0, EndTime: 20})
	if err != nil {
		t.Fatalf("QueryRows() no fields error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("missing field rows = %d, want 0", len(rows))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() after query error = %v", err)
	}
}

func TestEngineCompactsMultiplePartsAndReopens(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{
			Measurement: "cpu",
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(2)},
		},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Compact(ctx); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "cpu", StartTime: 0, EndTime: 20})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].Fields["v"].Float64 != 2 {
		t.Fatalf("value = %v, want 2", rows[0].Fields["v"].Float64)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eng, err = Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() after compact error = %v", err)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, 0)); err != nil {
		t.Fatalf("ApplyRetention() no-op error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() after reopen error = %v", err)
	}
}

func TestFlushFailureBeforeManifestDoesNotExposePart(t *testing.T) {
	dir := t.TempDir()
	opts := ShardOptions{
		Dir:                dir,
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 1,
	}
	shard, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	shard.testHooks.afterPartWriteBeforeManifest = func() error {
		return errors.New("stop before manifest")
	}
	if err := shard.Write(testResolvedPoint(1, 10, 1), true); err == nil {
		t.Fatal("Write() error = nil, want injected flush error")
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("OpenShard(reopen) error = %v", err)
	}
	got, err := reopened.Query(memtable.Query{Start: 0, End: 20})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("query count = %d, want WAL replayed data only", len(got))
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close(reopened) error = %v", err)
	}
}

func TestEngineWriteBatchUsesSingleWALFramePerShard(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := Open(ctx, model.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 10)
	for index := range 10 {
		points = append(points, testBatchPoint(int64(index)))
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	walPath := filepath.Join(dir, "data", "default", "autogen", "shards", "0", "wal", "000001.wal")
	frames, err := countWALFrames(walPath)
	if err != nil {
		t.Fatalf("countWALFrames() error = %v", err)
	}
	if frames != 1 {
		t.Fatalf("wal frames = %d, want 1", frames)
	}
}

func TestEngineWriteBatchAcrossShards(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := Open(ctx, model.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		testBatchPoint(1),
		testBatchPoint(int64(time.Hour) + 1),
		testBatchPoint(2),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "batch", StartTime: 0, EndTime: int64(2 * time.Hour)})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for _, start := range []string{"0", fmt.Sprint(int64(time.Hour))} {
		walPath := filepath.Join(dir, "data", "default", "autogen", "shards", start, "wal", "000001.wal")
		frames, err := countWALFrames(walPath)
		if err != nil {
			t.Fatalf("countWALFrames(%s) error = %v", start, err)
		}
		if frames != 1 {
			t.Fatalf("wal frames for shard %s = %d, want 1", start, frames)
		}
	}
}

func TestEngineFlushPersistsLatestWriteSeq(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(3)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(2)},
		},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].Fields["v"].Float64 != 2 {
		t.Fatalf("flushed value = %v, want latest write value 2", rows[0].Fields["v"].Float64)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestShardFlushFailureRestoresSnapshot(t *testing.T) {
	opts := ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 1,
	}
	shard, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	shard.testHooks.afterPartWriteBeforeManifest = func() error {
		return errors.New("stop before manifest")
	}
	err = shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 10, 1)}, false)
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want injected flush error")
	}
	got, queryErr := shard.Query(memtable.Query{Start: 0, End: 20})
	if queryErr != nil {
		t.Fatalf("Query() error = %v", queryErr)
	}
	if len(got) != 1 {
		t.Fatalf("query count after failed flush = %d, want 1", len(got))
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSizeTieredCompactionTriggersByPartCount(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
		Compaction: model.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 2,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 4 {
		point := model.Point{
			Measurement: "cpu",
			Timestamp:   int64(index),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(index))},
		}
		if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	shard := onlyShardForTest(t, eng)
	if len(shard.manifest.Parts) > 2 {
		t.Fatalf("part count = %d, want compacted to <=2", len(shard.manifest.Parts))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDirectorySizeAndCompactionSizeLimit(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(filePath, []byte("abcd"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	size, err := directorySize(dir)
	if err != nil {
		t.Fatalf("directorySize() error = %v", err)
	}
	if size != 4 {
		t.Fatalf("directorySize() = %d, want 4", size)
	}

	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
		Compaction: model.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 100,
			Level0SizeLimit: 1,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []model.Point{{
			Measurement: "cpu",
			Timestamp:   int64(index),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(index))},
		}}, model.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	shard := onlyShardForTest(t, eng)
	for _, part := range shard.manifest.Parts {
		if part.Level == 0 {
			t.Fatalf("manifest parts = %#v, want no level-0 parts after size-triggered compaction", shard.manifest.Parts)
		}
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := directorySize("bad\x00path"); err == nil {
		t.Fatal("directorySize(invalid) error = nil, want error")
	}
}

func TestBackgroundCompactionLifecycle(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
		Compaction: model.CompactionOptions{
			Enabled:            true,
			Level0PartLimit:    100,
			BackgroundInterval: 10 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 3 {
		if err := eng.Write(ctx, []model.Point{{
			Measurement: "bg",
			Timestamp:   int64(index),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(index))},
		}}, model.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	waitForTest(t, time.Second, func() bool {
		shard := onlyShardForTest(t, eng)
		return len(shard.manifest.Parts) == 1 && shard.manifest.Parts[0].Level == 1
	})
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCompactionSplitsOutputsByTargetBytes(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 4,
		Compaction: model.CompactionOptions{
			MaxOutputPartBytes: 80,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []model.Point{widePointForTest(int64(index))}, model.WriteOptions{Sync: true}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write() error = %v close = %v", err, closeErr)
		}
	}
	if err := eng.Compact(ctx); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	shard := onlyShardForTest(t, eng)
	if len(shard.manifest.Parts) <= 1 {
		t.Fatalf("part count = %d, want split compaction output", len(shard.manifest.Parts))
	}
	for _, part := range shard.manifest.Parts {
		if part.Level != 1 {
			t.Fatalf("part level = %d, want level 1", part.Level)
		}
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "wide", StartTime: 0, EndTime: 1})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 2 || len(rows[0].Fields) != 4 {
		t.Fatalf("rows = %#v, want two wide rows", rows)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionOutputCloseAndAbortCleanup(t *testing.T) {
	shard, _, err := OpenShard(ShardOptions{
		Dir:   t.TempDir(),
		Start: 0,
		End:   int64(time.Hour),
		Compaction: model.CompactionOptions{
			MaxOutputPartBytes: 80,
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	emptyOutput := newCompactionOutput(shard, 1)
	parts, metas, err := emptyOutput.close()
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("empty close() error = %v shard close = %v", err, closeErr)
	}
	if len(parts) != 0 || len(metas) != 0 {
		closeErr := shard.Close()
		t.Fatalf("empty close() = %d parts %d metas, want none close = %v", len(parts), len(metas), closeErr)
	}

	output := newCompactionOutput(shard, 1)
	first := []model.ColumnData{
		wideColumnForCompactionOutputTest(1, 1, "alpha"),
		wideColumnForCompactionOutputTest(1, 2, "beta"),
	}
	if err := output.addSeries(first); err != nil {
		closeErr := shard.Close()
		t.Fatalf("addSeries(first) error = %v close = %v", err, closeErr)
	}
	if len(output.metas) == 0 {
		closeErr := shard.Close()
		t.Fatalf("addSeries(first) did not roll output close = %v", closeErr)
	}
	closedPath := output.metas[0].Path
	if _, err := os.Stat(closedPath); err != nil {
		closeErr := shard.Close()
		t.Fatalf("closed output stat error = %v close = %v", err, closeErr)
	}
	if err := output.addSeries([]model.ColumnData{wideColumnForCompactionOutputTest(2, 1, "gamma")}); err != nil {
		closeErr := shard.Close()
		t.Fatalf("addSeries(second) error = %v close = %v", err, closeErr)
	}
	if output.writer == nil {
		closeErr := shard.Close()
		t.Fatalf("second writer is nil close = %v", closeErr)
	}
	openPath := filepath.Join(shard.opts.Dir, "sst-000003")
	if _, err := os.Stat(openPath); err != nil {
		closeErr := shard.Close()
		t.Fatalf("open output stat error = %v close = %v", err, closeErr)
	}
	if err := output.abort(); err != nil {
		closeErr := shard.Close()
		t.Fatalf("abort() error = %v close = %v", err, closeErr)
	}
	if _, err := os.Stat(closedPath); !os.IsNotExist(err) {
		closeErr := shard.Close()
		t.Fatalf("closed output stat after abort = %v, want not exist close = %v", err, closeErr)
	}
	if _, err := os.Stat(openPath); !os.IsNotExist(err) {
		closeErr := shard.Close()
		t.Fatalf("open output stat after abort = %v, want not exist close = %v", err, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestStreamingCompactionAbortsOpenOutputOnBatchQueryError(t *testing.T) {
	dir := t.TempDir()
	shard, _, err := OpenShard(ShardOptions{
		Dir:   dir,
		Start: 0,
		End:   int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}

	validColumns := make([]model.ColumnData, 0, streamingCompactionSeriesBatchSize)
	for seriesID := 1; seriesID <= streamingCompactionSeriesBatchSize; seriesID++ {
		validColumns = append(validColumns, streamCompactionColumnForTest(uint64(seriesID)))
	}
	validMeta, err := sstable.WritePart(dir, 0, "sst-000001", validColumns)
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("WritePart(valid) error = %v close = %v", err, closeErr)
	}
	validPart, err := sstable.OpenPart(validMeta.Path)
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("OpenPart(valid) error = %v close = %v", err, closeErr)
	}

	corruptMeta, err := sstable.WritePart(dir, 0, "sst-000002", []model.ColumnData{
		streamCompactionColumnForTest(uint64(streamingCompactionSeriesBatchSize + 1)),
	})
	if err != nil {
		validCloseErr := validPart.Close()
		closeErr := shard.Close()
		t.Fatalf("WritePart(corrupt) error = %v valid close = %v shard close = %v", err, validCloseErr, closeErr)
	}
	corruptPart, err := sstable.OpenPart(corruptMeta.Path)
	if err != nil {
		validCloseErr := validPart.Close()
		closeErr := shard.Close()
		t.Fatalf("OpenPart(corrupt) error = %v valid close = %v shard close = %v", err, validCloseErr, closeErr)
	}
	corruptValuesFileAtPath(t, corruptMeta.Path)

	_, _, err = shard.writeStreamingCompactionOutputsLocked(1, []partReader{validPart, corruptPart})
	if err == nil {
		validCloseErr := validPart.Close()
		corruptCloseErr := corruptPart.Close()
		closeErr := shard.Close()
		t.Fatalf(
			"writeStreamingCompactionOutputsLocked() error = nil, want corrupt query error valid close = %v corrupt close = %v shard close = %v",
			validCloseErr,
			corruptCloseErr,
			closeErr,
		)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "sst-000003")); !os.IsNotExist(statErr) {
		validCloseErr := validPart.Close()
		corruptCloseErr := corruptPart.Close()
		closeErr := shard.Close()
		t.Fatalf(
			"stream output stat = %v, want aborted output removed valid close = %v corrupt close = %v shard close = %v",
			statErr,
			validCloseErr,
			corruptCloseErr,
			closeErr,
		)
	}
	if err := validPart.Close(); err != nil {
		corruptCloseErr := corruptPart.Close()
		closeErr := shard.Close()
		t.Fatalf("Close(valid) error = %v corrupt close = %v shard close = %v", err, corruptCloseErr, closeErr)
	}
	if err := corruptPart.Close(); err != nil {
		closeErr := shard.Close()
		t.Fatalf("Close(corrupt) error = %v shard close = %v", err, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCompactionStreamsOneSeriesAtATime(t *testing.T) {
	dir := t.TempDir()
	shard, _, err := OpenShard(ShardOptions{
		Dir:   dir,
		Start: 0,
		End:   int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	partOneMeta, err := sstable.WritePart(dir, 0, "sst-000001", []model.ColumnData{
		compactionColumnForSeriesTest(1, []model.VersionedSample{
			{Timestamp: 1, WriteSeq: 1, Value: model.Float64Value(1)},
			{Timestamp: 2, WriteSeq: 1, Value: model.Float64Value(2)},
		}),
		compactionColumnForSeriesTest(2, []model.VersionedSample{
			{Timestamp: 1, WriteSeq: 1, Value: model.Float64Value(10)},
		}),
	})
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("WritePart(one) error = %v close = %v", err, closeErr)
	}
	partTwoMeta, err := sstable.WritePart(dir, 0, "sst-000002", []model.ColumnData{
		compactionColumnForSeriesTest(1, []model.VersionedSample{
			{Timestamp: 2, WriteSeq: 9, Value: model.Float64Value(20)},
			{Timestamp: 3, WriteSeq: 9, Value: model.Float64Value(30)},
		}),
		compactionColumnForSeriesTest(2, []model.VersionedSample{
			{Timestamp: 2, WriteSeq: 1, Value: model.Float64Value(11)},
		}),
	})
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("WritePart(two) error = %v close = %v", err, closeErr)
	}
	partOne, err := sstable.OpenPart(partOneMeta.Path)
	if err != nil {
		closeErr := shard.Close()
		t.Fatalf("OpenPart(one) error = %v close = %v", err, closeErr)
	}
	partTwo, err := sstable.OpenPart(partTwoMeta.Path)
	if err != nil {
		closeErr := errors.Join(partOne.Close(), shard.Close())
		t.Fatalf("OpenPart(two) error = %v close = %v", err, closeErr)
	}
	inputs, err := newCompactionInputs(defaultPartManager{}, []partReader{partOne, partTwo}, 0, int64(time.Hour))
	if err != nil {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("newCompactionInputs() error = %v close = %v", err, closeErr)
	}
	shard.tombstones = []model.Tombstone{{SeriesIDs: []uint64{1}, StartTime: 1, EndTime: 1}}
	columns, err := shard.queryCompactionSeries(inputs, 1)
	if err != nil {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("queryCompactionSeries() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || columns[0].SeriesID != 1 {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("columns = %#v, want only series 1 close = %v", columns, closeErr)
	}
	if got := columns[0].Samples; len(got) != 2 || got[0].Timestamp != 2 || got[0].Value.Float64 != 20 || got[1].Timestamp != 3 {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("series 1 samples = %#v, want tombstoned ts=1 and newest ts=2 close = %v", got, closeErr)
	}
	seriesTwo, err := shard.queryCompactionSeries(inputs, 2)
	if err != nil {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("queryCompactionSeries(series 2) error = %v close = %v", err, closeErr)
	}
	if len(seriesTwo) != 1 || seriesTwo[0].SeriesID != 2 || len(seriesTwo[0].Samples) != 2 {
		closeErr := errors.Join(partOne.Close(), partTwo.Close(), shard.Close())
		t.Fatalf("series 2 columns = %#v, want untouched two samples close = %v", seriesTwo, closeErr)
	}
	if err := errors.Join(partOne.Close(), partTwo.Close(), shard.Close()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenShardCleansOrphanParts(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{Path: t.TempDir(), ShardDuration: time.Hour, MemTableMaxSamples: 1}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "orphan",
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	orphanPath := filepath.Join(shardDir(opts.Path, "default", "autogen", 0), "sst-999999")
	_, err = sstable.WritePart(filepath.Dir(orphanPath), 0, filepath.Base(orphanPath), []model.ColumnData{
		columnForOrphanTest(),
	})
	if err != nil {
		t.Fatalf("WritePart(orphan) error = %v", err)
	}
	reopened, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("reopen Open() error = %v", err)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		closeErr := reopened.Close(ctx)
		t.Fatalf("orphan stat error = %v, want not exist close = %v", err, closeErr)
	}
	if errs := reopened.MaintenanceErrors(ctx); len(errs) != 0 {
		closeErr := reopened.Close(ctx)
		t.Fatalf("MaintenanceErrors() = %v, want none close = %v", errs, closeErr)
	}
	if err := reopened.Close(ctx); err != nil {
		t.Fatalf("Close(reopened) error = %v", err)
	}
}

func TestMetadataAPIDropsDatabaseData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	eng, err := Open(ctx, model.Options{Path: dir, ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.CreateDatabase(ctx, "metrics"); err != nil {
		t.Fatalf("CreateDatabase() error = %v", err)
	}
	if err := eng.CreateRetentionPolicy(ctx, "metrics", model.RetentionPolicy{Name: "hot"}); err != nil {
		t.Fatalf("CreateRetentionPolicy() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Timestamp:       1,
		Fields:          map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	fields, err := eng.ListFields(ctx, "metrics", "cpu")
	if err != nil {
		t.Fatalf("ListFields() error = %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("field count = %d, want 1", len(fields))
	}
	policies, err := eng.ListRetentionPolicies(ctx, "metrics")
	if err != nil {
		t.Fatalf("ListRetentionPolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "hot" {
		t.Fatalf("policies = %#v, want hot", policies)
	}
	measurements, err := eng.ListMeasurements(ctx, "metrics")
	if err != nil {
		t.Fatalf("ListMeasurements() error = %v", err)
	}
	if len(measurements) != 1 || measurements[0] != "cpu" {
		t.Fatalf("measurements = %#v, want cpu", measurements)
	}
	series, err := eng.ListSeries(ctx, "metrics", "cpu", nil)
	if err != nil {
		t.Fatalf("ListSeries() error = %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("series count = %d, want 1", len(series))
	}
	if err := eng.DropDatabase(ctx, "metrics"); err != nil {
		t.Fatalf("DropDatabase() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		StartTime:       0,
		EndTime:         10,
	})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows after drop = %#v, want none", rows)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "metrics")); !os.IsNotExist(err) {
		t.Fatalf("data dir stat = %v, want not exist", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestShardDeleteRangeReplaysAndCompactsTombstone(t *testing.T) {
	opts := ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 10,
	}
	shard, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	points := []model.ResolvedPoint{
		testResolvedPoint(1, 10, 1),
		testResolvedPoint(1, 20, 2),
		testResolvedPoint(1, 30, 3),
	}
	if err := shard.WriteBatch(points, true); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}
	if err := shard.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	tombstone := model.Tombstone{
		SeriesIDs: []uint64{1},
		FieldIDs:  []uint32{1},
		StartTime: 15,
		EndTime:   25,
		WriteSeq:  4,
	}
	if err := shard.DeleteRange(tombstone, true); err != nil {
		t.Fatalf("DeleteRange() error = %v", err)
	}
	got, err := shard.Query(memtable.Query{Start: 0, End: 40})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 2 {
		t.Fatalf("query after tombstone = %#v, want two samples", got)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("reopen OpenShard() error = %v", err)
	}
	got, err = reopened.Query(memtable.Query{Start: 0, End: 40})
	if err != nil {
		t.Fatalf("Query(reopened) error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 2 {
		t.Fatalf("reopened query after tombstone = %#v, want two samples", got)
	}
	if err := reopened.Compact(); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if len(reopened.tombstones) != 0 {
		t.Fatalf("tombstone count after compact = %d, want 0", len(reopened.tombstones))
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close(reopened) error = %v", err)
	}
	finalShard, _, err := OpenShard(opts)
	if err != nil {
		t.Fatalf("final OpenShard() error = %v", err)
	}
	got, err = finalShard.Query(memtable.Query{Start: 0, End: 40})
	closeErr := finalShard.Close()
	if err != nil {
		t.Fatalf("Query(final) error = %v close = %v", err, closeErr)
	}
	if closeErr != nil {
		t.Fatalf("Close(final) error = %v", closeErr)
	}
	if len(got) != 1 || len(got[0].Samples) != 2 {
		t.Fatalf("final query after compact = %#v, want two samples", got)
	}
}

func TestEngineRetentionKeepsActiveShard(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		Retention:          time.Hour,
		MemTableMaxSamples: 10,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Timestamp:   int64(30 * time.Minute),
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(45*time.Minute))); err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenRejectsEmptyPathAndShardStartFloorsNegativeTime(t *testing.T) {
	if _, err := Open(context.Background(), model.Options{}); err == nil {
		t.Fatal("Open() error = nil, want empty path error")
	}
	opts := normalizeOptions(model.Options{Compaction: model.CompactionOptions{Enabled: true}})
	if opts.Compaction.Level0PartLimit != 4 {
		t.Fatalf("default Level0PartLimit = %d, want 4", opts.Compaction.Level0PartLimit)
	}
	if got := shardStart(-1, time.Hour); got != -int64(time.Hour) {
		t.Fatalf("shardStart(-1) = %d, want %d", got, -int64(time.Hour))
	}
	shard := &Shard{}
	if err := shard.Close(); err != nil {
		t.Fatalf("empty shard Close() error = %v", err)
	}
	if partNumber("bad") != 0 {
		t.Fatal("partNumber(bad) != 0")
	}
}

func TestEngineEmptyWriteInvalidWriteAndRetentionNoop(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, nil, model.WriteOptions{}); err != nil {
		t.Fatalf("Write(nil) error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{Measurement: "bad"}}, model.WriteOptions{}); err == nil {
		t.Fatal("Write(invalid) error = nil, want error")
	}
	if err := eng.ApplyRetention(ctx, time.Now()); err != nil {
		t.Fatalf("ApplyRetention() without retention error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestEngineEmptyLifecycle(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{Path: t.TempDir(), Retention: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if err := eng.Compact(ctx); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if err := eng.ApplyRetention(ctx, time.Now()); err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestEngineQueryPropagatesPartCorruption(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Timestamp:   10,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	corruptValuesFile(t, opts.Path)
	if _, err := eng.QueryRows(ctx, model.Query{Measurement: "cpu", StartTime: 0, EndTime: 20}); err == nil {
		t.Fatal("QueryRows() corruption error = nil, want error")
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func corruptValuesFile(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "data", defaultDatabase, defaultRetentionPolicy, "shards", "0", "sst-000001", "values.bin")
	corruptValuesFilePath(t, path)
}

func corruptValuesFileAtPath(t *testing.T, partPath string) {
	t.Helper()
	corruptValuesFilePath(t, filepath.Join(partPath, "values.bin"))
}

func corruptValuesFilePath(t *testing.T, path string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(values) error = %v", err)
	}
	if _, err := file.WriteAt([]byte{0xff}, 12); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt(values) error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(values) error = %v", err)
	}
}

func streamCompactionColumnForTest(seriesID uint64) model.ColumnData {
	timestamp := int64(seriesID)
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: timestamp, WriteSeq: seriesID, Value: model.Float64Value(float64(seriesID))},
		},
	}
}

func compactionColumnForSeriesTest(seriesID uint64, samples []model.VersionedSample) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}
}

func wideColumnForCompactionOutputTest(seriesID uint64, fieldID uint32, value string) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldString,
		Samples: []model.VersionedSample{
			{Timestamp: int64(fieldID), WriteSeq: uint64(fieldID), Value: model.StringValue(value)},
		},
	}
}

func testResolvedPoint(seriesID uint64, timestamp int64, writeSeq uint64) model.ResolvedPoint {
	return model.ResolvedPoint{
		SeriesID:  seriesID,
		Timestamp: timestamp,
		WriteSeq:  writeSeq,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "v", Type: model.FieldFloat64, Value: model.Float64Value(float64(writeSeq))},
		},
	}
}

func testBatchPoint(timestamp int64) model.Point {
	return model.Point{
		Measurement: "batch",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   timestamp,
		Fields: map[string]model.FieldValue{
			"v": model.Float64Value(float64(timestamp)),
		},
	}
}

func widePointForTest(timestamp int64) model.Point {
	return model.Point{
		Measurement: "wide",
		Timestamp:   timestamp,
		Fields: map[string]model.FieldValue{
			"f0": model.Float64Value(float64(timestamp)),
			"f1": model.Int64Value(timestamp),
			"f2": model.StringValue("same"),
			"f3": model.BoolValue(timestamp%2 == 0),
		},
	}
}

func columnForOrphanTest() model.ColumnData {
	return model.ColumnData{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 1, WriteSeq: 99, Value: model.Float64Value(99)},
		},
	}
}

func waitForTest(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func countWALFrames(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	count, readErr := countOpenWALFrames(file)
	closeErr := file.Close()
	if readErr != nil {
		return 0, readErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	return count, nil
}

func countOpenWALFrames(reader io.Reader) (int, error) {
	count := 0
	header := make([]byte, 4)
	for {
		if _, err := io.ReadFull(reader, header); err != nil {
			if err == io.EOF {
				return count, nil
			}
			return 0, err
		}
		length := binary.BigEndian.Uint32(header)
		if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
			return 0, err
		}
		count++
	}
}

func onlyShardForTest(t *testing.T, eng *Engine) *Shard {
	t.Helper()
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if len(eng.shards) != 1 {
		t.Fatalf("shard count = %d, want 1", len(eng.shards))
	}
	for _, shard := range eng.shards {
		return shard
	}
	t.Fatal("no shard found")
	return nil
}

func TestMergeColumnDataKeepsNewestSequence(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 2, Value: model.Float64Value(3)},
			},
		},
	}
	merged := mergeColumnData(columns)
	if len(merged) != 1 {
		t.Fatalf("column count = %d, want 1", len(merged))
	}
	if merged[0].Samples[0].Value.Float64 != 3 {
		t.Fatalf("merged value = %v, want 3", merged[0].Samples[0].Value.Float64)
	}
	rows := []model.Row{
		{SeriesID: 2, Timestamp: 1},
		{SeriesID: 1, Timestamp: 2},
		{SeriesID: 1, Timestamp: 1},
	}
	sortRows(rows)
	if rows[0].SeriesID != 1 || rows[0].Timestamp != 1 {
		t.Fatalf("first row = (%d,%d), want (1,1)", rows[0].SeriesID, rows[0].Timestamp)
	}
	if shardMatches(&Shard{opts: ShardOptions{Database: "a", RetentionPolicy: "b"}}, model.Query{Database: "x", RetentionPolicy: "b"}) {
		t.Fatal("shardMatches() matched wrong database")
	}
	empty := &Shard{mem: memStoreForTest()}
	if err := empty.Flush(); err != nil {
		t.Fatalf("empty Flush() error = %v", err)
	}
}

func TestShardUsesInjectedStoragePorts(t *testing.T) {
	mem := &fakeMemStore{
		snapshot: &fakeMemSnapshot{
			columns: []model.ColumnData{columnForMergeTest(1, 1, 0, 1)},
			samples: 1,
		},
		samples: 1,
	}
	walLog := &fakeWalStore{}
	parts := &fakePartManager{}
	files := &fakeFileOps{}
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                100,
		MemTableMaxSamples: 10,
		deps: shardDeps{
			openWAL: func(string, model.WALOptions) (walStore, error) {
				return walLog, nil
			},
			newMem: func() memStore {
				return mem
			},
			parts: parts,
			files: files,
		},
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	if err := shard.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !parts.writePartCalled || !parts.openPartCalled || !parts.writeManifestCalled {
		t.Fatalf("part manager calls write=%v open=%v manifest=%v, want all true",
			parts.writePartCalled, parts.openPartCalled, parts.writeManifestCalled)
	}
	if !walLog.checkpointCalled {
		t.Fatal("wal checkpoint called = false, want true")
	}
	if !mem.snapshotReleased {
		t.Fatal("snapshot released = false, want true")
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !walLog.closed {
		t.Fatal("wal closed = false, want true")
	}
	if files.removeAllCalls != 0 {
		t.Fatalf("file RemoveAll calls = %d, want 0", files.removeAllCalls)
	}
}

func TestColumnsToRowsFallsBackForUnalignedColumns(t *testing.T) {
	columns := []model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			FieldName:   "a",
			Timestamps:  []int64{1, 3},
			Values:      []model.FieldValue{model.Int64Value(1), model.Int64Value(3)},
		},
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			FieldName:   "b",
			Timestamps:  []int64{2, 3},
			Values:      []model.FieldValue{model.Int64Value(2), model.Int64Value(30)},
		},
	}
	rows := columnsToRows(columns)
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	if rows[2].Fields["a"].Int64 != 3 || rows[2].Fields["b"].Int64 != 30 {
		t.Fatalf("merged row = %#v, want fields a=3 b=30", rows[2])
	}
}

func TestIteratorsReturnZeroBeforeNextAndAfterClose(t *testing.T) {
	columns := &columnIterator{decorated: []model.ColumnSeries{{SeriesID: 1}}}
	if got := columns.Column(); got.SeriesID != 0 {
		t.Fatalf("Column(before Next) = %#v, want zero", got)
	}
	if !columns.Next() {
		t.Fatal("column Next() = false, want true")
	}
	if got := columns.Column(); got.SeriesID != 1 {
		t.Fatalf("Column() = %#v, want series 1", got)
	}
	if err := columns.Close(); err != nil {
		t.Fatalf("column Close() error = %v", err)
	}

	rows := &rowIterator{rows: []model.Row{{SeriesID: 2}}}
	if got := rows.Row(); got.SeriesID != 0 {
		t.Fatalf("Row(before Next) = %#v, want zero", got)
	}
	if !rows.Next() {
		t.Fatal("row Next() = false, want true")
	}
	if got := rows.Row(); got.SeriesID != 2 {
		t.Fatalf("Row() = %#v, want series 2", got)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("row Close() error = %v", err)
	}
}

func TestQueryColumnIteratorDecoratesLazily(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields: map[string]model.FieldValue{
			"f0": model.Float64Value(1),
			"f1": model.Float64Value(2),
		},
	}}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	decorations := 0
	decorateColumnDataHook = func() {
		decorations++
	}
	defer func() {
		decorateColumnDataHook = nil
	}()
	iter, err := eng.QueryColumnIterator(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     10,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumnIterator() error = %v close = %v", err, closeErr)
	}
	if decorations != 0 {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("decorations after iterator creation = %d, want 0 close = %v", decorations, closeErr)
	}
	if !iter.Next() {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("Next() = false, want true close = %v", closeErr)
	}
	if decorations != 0 {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("decorations after Next = %d, want 0 close = %v", decorations, closeErr)
	}
	if got := iter.Column(); got.SeriesID == 0 {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("Column() = %#v, want decorated column close = %v", got, closeErr)
	}
	if decorations != 1 {
		closeErr := errors.Join(iter.Close(), eng.Close(ctx))
		t.Fatalf("decorations after Column = %d, want 1 close = %v", decorations, closeErr)
	}
	if err := iter.Close(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("iterator Close() error = %v engine close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestApplyTombstonesWithDefaultWriteSeqAndNoMatches(t *testing.T) {
	got := applyTombstones(tombstoneColumnsForTest(), []model.Tombstone{{StartTime: 0, EndTime: 20}})
	if len(got) != 0 {
		t.Fatalf("default tombstone result = %#v, want deleted", got)
	}
	got = applyTombstones(tombstoneColumnsForTest(), []model.Tombstone{{SeriesIDs: []uint64{2}, StartTime: 0, EndTime: 20}})
	if len(got) != 1 {
		t.Fatalf("non-matching tombstone result = %#v, want kept", got)
	}
}

func tombstoneColumnsForTest() []model.ColumnData {
	return []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 10, WriteSeq: 100, Value: model.Float64Value(1)},
		},
	}}
}

func TestMergeColumnDataOrderedFastPathAllocations(t *testing.T) {
	columns := []model.ColumnData{
		columnForMergeTest(2, 1, 0, 100),
		columnForMergeTest(1, 1, 0, 100),
		columnForMergeTest(1, 1, 50, 100),
		columnForMergeTest(1, 2, 0, 100),
	}
	columns[2].Samples[0] = model.VersionedSample{
		Timestamp: 50,
		WriteSeq:  999,
		Value:     model.Float64Value(999),
	}
	allocs := testing.AllocsPerRun(100, func() {
		merged := mergeColumnData(columns)
		if len(merged) != 3 {
			t.Fatalf("merged column count = %d, want 3", len(merged))
		}
		if merged[0].SeriesID != 1 || merged[0].FieldID != 1 {
			t.Fatalf("first column = (%d,%d), want (1,1)", merged[0].SeriesID, merged[0].FieldID)
		}
		if len(merged[0].Samples) != 150 {
			t.Fatalf("merged samples = %d, want 150", len(merged[0].Samples))
		}
		if merged[0].Samples[50].Value.Float64 != 999 {
			t.Fatalf("latest value at timestamp 50 = %v, want 999", merged[0].Samples[50].Value.Float64)
		}
	})
	if allocs > 20 {
		t.Fatalf("mergeColumnData ordered allocs/run = %.2f, want <= 20", allocs)
	}
}

func TestForEachCompactionSeriesGroupVisitsSortedGroups(t *testing.T) {
	columns := []model.ColumnData{
		columnForMergeTest(2, 1, 0, 1),
		columnForMergeTest(1, 2, 0, 1),
		columnForMergeTest(1, 1, 0, 1),
	}
	var got []uint64
	if err := forEachCompactionSeriesGroup(columns, func(group []model.ColumnData) error {
		got = append(got, group[0].SeriesID)
		for _, column := range group {
			if column.SeriesID != group[0].SeriesID {
				t.Fatalf("mixed series group = %#v", group)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("forEachCompactionSeriesGroup() error = %v", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("visited series = %v, want [1 2]", got)
	}
	if err := forEachCompactionSeriesGroup(nil, func(group []model.ColumnData) error {
		t.Fatalf("unexpected empty group visit: %#v", group)
		return nil
	}); err != nil {
		t.Fatalf("forEachCompactionSeriesGroup(empty) error = %v", err)
	}
	stopErr := errors.New("stop")
	err := forEachCompactionSeriesGroup(columns, func(group []model.ColumnData) error {
		return stopErr
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("forEachCompactionSeriesGroup(error) = %v, want %v", err, stopErr)
	}
}

func TestMergeColumnDataSingleOrderedColumnKeepsNewestSequence(t *testing.T) {
	merged := mergeColumnData([]model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldInt64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Int64Value(1)},
				{Timestamp: 10, WriteSeq: 3, Value: model.Int64Value(3)},
				{Timestamp: 20, WriteSeq: 2, Value: model.Int64Value(2)},
			},
		},
	})
	if len(merged) != 1 {
		t.Fatalf("merged column count = %d, want 1", len(merged))
	}
	got := merged[0].Samples
	if len(got) != 2 {
		t.Fatalf("sample count = %d, want 2", len(got))
	}
	if got[0].Timestamp != 10 || got[0].Value.Int64 != 3 {
		t.Fatalf("first sample = %#v, want timestamp 10 value 3", got[0])
	}
}

func TestMergeColumnDataUnorderedSamplesUseFallback(t *testing.T) {
	merged := mergeColumnData([]model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 20, WriteSeq: 1, Value: model.Float64Value(2)},
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 3, Value: model.Float64Value(3)},
			},
		},
	})
	if len(merged) != 1 {
		t.Fatalf("merged column count = %d, want 1", len(merged))
	}
	got := merged[0].Samples
	if len(got) != 2 {
		t.Fatalf("sample count = %d, want 2", len(got))
	}
	if got[0].Timestamp != 10 || got[0].Value.Float64 != 3 {
		t.Fatalf("first sample = %#v, want timestamp 10 value 3", got[0])
	}
	if got[1].Timestamp != 20 || got[1].Value.Float64 != 2 {
		t.Fatalf("second sample = %#v, want timestamp 20 value 2", got[1])
	}
}

func columnForMergeTest(seriesID uint64, fieldID uint32, start int64, count int) model.ColumnData {
	samples := make([]model.VersionedSample, 0, count)
	for index := 0; index < count; index++ {
		timestamp := start + int64(index)
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(timestamp)),
		})
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}
}

func memStoreForTest() memStore {
	return memTableStore{inner: memtable.New()}
}

func TestMemTableStoreAdapterSnapshotAndRestore(t *testing.T) {
	store := memTableStore{inner: memtable.New()}
	point := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 1,
		Fields: []model.ResolvedField{
			{FieldID: 1, Type: model.FieldFloat64, Value: model.Float64Value(1)},
		},
	}
	if err := store.Apply(point); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	snapshot := store.Snapshot()
	if snapshot.SampleCount() != 1 {
		t.Fatalf("Snapshot sample count = %d, want 1", snapshot.SampleCount())
	}
	reset := store.SnapshotAndReset()
	if store.SampleCount() != 0 {
		t.Fatalf("SampleCount after reset = %d, want 0", store.SampleCount())
	}
	store.Restore(reset)
	if store.SampleCount() != 1 {
		t.Fatalf("SampleCount after restore = %d, want 1", store.SampleCount())
	}
	store.Restore(&fakeMemSnapshot{samples: 10})
	if store.SampleCount() != 1 {
		t.Fatalf("SampleCount after unsupported restore = %d, want unchanged 1", store.SampleCount())
	}
	snapshot.Release()
}

type fakeWalStore struct {
	points           []model.ResolvedPoint
	tombstones       []model.Tombstone
	checkpointCalled bool
	closed           bool
}

func (f *fakeWalStore) Append(points []model.ResolvedPoint, _ bool) error {
	f.points = append(f.points, points...)
	return nil
}

func (f *fakeWalStore) AppendTombstones(tombstones []model.Tombstone, _ bool) error {
	f.tombstones = append(f.tombstones, tombstones...)
	return nil
}

func (f *fakeWalStore) ReplayRecords() ([]wal.Record, error) {
	return nil, nil
}

func (f *fakeWalStore) Checkpoint() error {
	f.checkpointCalled = true
	return nil
}

func (f *fakeWalStore) Close() error {
	f.closed = true
	return nil
}

type fakeMemStore struct {
	snapshot         *fakeMemSnapshot
	samples          int
	snapshotReleased bool
}

func (f *fakeMemStore) Apply(point model.ResolvedPoint) error {
	f.samples += len(point.Fields)
	return nil
}

func (f *fakeMemStore) ApplyBatch(points []model.ResolvedPoint) error {
	for _, point := range points {
		f.samples += len(point.Fields)
	}
	return nil
}

func (f *fakeMemStore) SampleCount() int {
	return f.samples
}

func (f *fakeMemStore) SnapshotAndReset() memSnapshot {
	f.samples = 0
	f.snapshot.onRelease = func() {
		f.snapshotReleased = true
	}
	return f.snapshot
}

func (f *fakeMemStore) Snapshot() memSnapshot {
	return f.snapshot
}

func (f *fakeMemStore) Query(memtable.Query) []model.ColumnData {
	return f.snapshot.columns
}

func (f *fakeMemStore) Restore(snapshot memSnapshot) {
	if snapshot != nil {
		f.samples = snapshot.SampleCount()
	}
}

type fakeMemSnapshot struct {
	columns   []model.ColumnData
	samples   int
	onRelease func()
}

func (f *fakeMemSnapshot) Columns(memtable.Query) []model.ColumnData {
	return f.columns
}

func (f *fakeMemSnapshot) Query(query memtable.Query) []model.ColumnData {
	return f.Columns(query)
}

func (f *fakeMemSnapshot) SampleCount() int {
	return f.samples
}

func (f *fakeMemSnapshot) Release() {
	if f.onRelease != nil {
		f.onRelease()
	}
}

type fakePartManager struct {
	writePartCalled     bool
	openPartCalled      bool
	writeManifestCalled bool
}

func (f *fakePartManager) LoadManifest(string) (sstable.Manifest, error) {
	return sstable.Manifest{}, nil
}

func (f *fakePartManager) WriteManifest(_ string, _ sstable.Manifest) error {
	f.writeManifestCalled = true
	return nil
}

func (f *fakePartManager) OpenPart(string) (partReader, error) {
	f.openPartCalled = true
	return fakePartReader{meta: sstable.PartMeta{ID: "sst-000001", Path: "fake"}}, nil
}

func (f *fakePartManager) WritePart(
	_ string,
	_ int,
	id string,
	columns []model.ColumnData,
	_ sstable.WriteOptions,
) (sstable.PartMeta, error) {
	f.writePartCalled = true
	return sstable.PartMeta{
		ID:          id,
		Path:        "fake",
		MinTime:     columns[0].Samples[0].Timestamp,
		MaxTime:     columns[0].Samples[0].Timestamp,
		MinSeriesID: columns[0].SeriesID,
		MaxSeriesID: columns[0].SeriesID,
		SeriesCount: 1,
		RowsCount:   1,
	}, nil
}

func (f *fakePartManager) NewWriter(string, int, string, sstable.WriteOptions) (partWriter, error) {
	return &fakePartWriter{}, nil
}

func (f *fakePartManager) NewSeriesBatchReader(partReader, sstable.Query) (seriesBatchReader, error) {
	return fakeSeriesBatchReader{}, nil
}

type fakePartReader struct {
	meta sstable.PartMeta
}

func (f fakePartReader) Close() error {
	return nil
}

func (f fakePartReader) Meta() sstable.PartMeta {
	return f.meta
}

func (f fakePartReader) Query(sstable.Query) ([]model.ColumnData, error) {
	return nil, nil
}

func (f fakePartReader) QuerySeriesIDs(sstable.Query, []uint64) ([]model.ColumnData, error) {
	return nil, nil
}

func (f fakePartReader) SeriesIDs(sstable.Query) ([]uint64, error) {
	return nil, nil
}

type fakePartWriter struct{}

func (f *fakePartWriter) AddSeries([]model.ColumnData) error {
	return nil
}

func (f *fakePartWriter) Close() (sstable.PartMeta, error) {
	return sstable.PartMeta{ID: "sst-000002", Path: "fake-2"}, nil
}

func (f *fakePartWriter) Abort() error {
	return nil
}

type fakeSeriesBatchReader struct{}

func (fakeSeriesBatchReader) SeriesIDs() []uint64 {
	return nil
}

func (fakeSeriesBatchReader) SeriesCount() int {
	return 0
}

func (fakeSeriesBatchReader) AppendSeriesIDs(dst []uint64) []uint64 {
	return dst
}

func (fakeSeriesBatchReader) QuerySeriesIDs([]uint64) ([]model.ColumnData, error) {
	return nil, nil
}

func (fakeSeriesBatchReader) QuerySeriesID(uint64) ([]model.ColumnData, error) {
	return nil, nil
}

type fakeFileOps struct {
	removeAllCalls int
}

func (f *fakeFileOps) RemoveAll(string) error {
	f.removeAllCalls++
	return nil
}

func TestDecorateColumnsSkipsMissingCatalogEntries(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("catalog.Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("catalog.Close() error = %v", err)
		}
	}()
	got := decorateColumns([]model.ColumnData{
		{SeriesID: 999, FieldID: 1},
	}, cat.Snapshot())
	if len(got) != 0 {
		t.Fatalf("decorated count = %d, want 0", len(got))
	}
}
