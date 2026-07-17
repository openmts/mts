package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
)

func TestQueryIteratorMakesDropDatabaseReturnBusy(t *testing.T) {
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
		Timestamp:   10,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// 直接对 shard 打开扫描流，确保读引用在 Close 前一直持有。
	var shard *Shard
	for _, item := range eng.shards {
		shard = item
		break
	}
	if shard == nil {
		closeTestEngine(t, ctx, eng)
		t.Fatal("no shard after write")
	}
	stream, err := shard.ScanColumns(memtable.Query{Start: 0, End: 100})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("ScanColumns() error = %v", err)
	}

	err = eng.DropDatabase(ctx, "default")
	if !errors.Is(err, ErrShardBusy) {
		_ = stream.Close()
		closeTestEngine(t, ctx, eng)
		t.Fatalf("DropDatabase() error = %v, want ErrShardBusy", err)
	}

	if err := stream.Close(); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("stream Close() error = %v", err)
	}
	if err := eng.DropDatabase(ctx, "default"); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("DropDatabase() after stream close error = %v", err)
	}
	closeTestEngine(t, ctx, eng)
}

func TestShardWriteFlushAndScanNoDoubleCount(t *testing.T) {
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	for i := 0; i < 50; i++ {
		if err := shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, int64(i), uint64(i+1))}, false); err != nil {
			_ = shard.Close()
			t.Fatalf("WriteBatch(%d) error = %v", i, err)
		}
	}
	stream, err := shard.ScanColumns(memtable.Query{Start: 0, End: 100})
	if err != nil {
		_ = shard.Close()
		t.Fatalf("ScanColumns() error = %v", err)
	}
	// flush while scan open should not panic; compaction may skip
	if err := shard.Flush(); err != nil {
		_ = stream.Close()
		_ = shard.Close()
		t.Fatalf("Flush() during scan error = %v", err)
	}
	total := 0
	for stream.Next() {
		total += len(stream.ColumnData().Samples)
	}
	if err := stream.Err(); err != nil {
		_ = stream.Close()
		_ = shard.Close()
		t.Fatalf("stream err = %v", err)
	}
	if err := stream.Close(); err != nil {
		_ = shard.Close()
		t.Fatalf("stream Close() error = %v", err)
	}
	if total < 50 {
		_ = shard.Close()
		t.Fatalf("scanned samples = %d, want >= 50", total)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestShardCloseBusyWhileScanOpen(t *testing.T) {
	shard, _, err := OpenShard(ShardOptions{
		Dir:                t.TempDir(),
		Start:              0,
		End:                int64(time.Hour),
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	if err := shard.WriteBatch([]model.ResolvedPoint{testResolvedPoint(1, 1, 1)}, false); err != nil {
		_ = shard.Close()
		t.Fatalf("WriteBatch() error = %v", err)
	}
	stream, err := shard.ScanColumns(memtable.Query{Start: 0, End: 10})
	if err != nil {
		_ = shard.Close()
		t.Fatalf("ScanColumns() error = %v", err)
	}
	if err := shard.Close(); !errors.Is(err, ErrShardBusy) {
		_ = stream.Close()
		t.Fatalf("Close() error = %v, want ErrShardBusy", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("stream Close() error = %v", err)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() after stream close error = %v", err)
	}
}
