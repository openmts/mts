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
	empty := &Shard{mem: memtable.New()}
	if err := empty.Flush(); err != nil {
		t.Fatalf("empty Flush() error = %v", err)
	}
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
	eng := &Engine{catalog: cat}
	got := eng.decorateColumns([]model.ColumnData{
		{SeriesID: 999, FieldID: 1},
	})
	if len(got) != 0 {
		t.Fatalf("decorated count = %d, want 0", len(got))
	}
}
