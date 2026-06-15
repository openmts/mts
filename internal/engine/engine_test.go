package engine

import (
	"context"
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
