package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestShouldFlushMemTableOnDisorderRatio(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:                            t.TempDir(),
		ShardDuration:                   time.Hour,
		MemTableMaxSamples:              100000,
		MemTableDisorderFlushRatio:      0.5,
		MemTableDisorderFlushMinSamples: 4,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// 有序 2 + 乱序 2 => ratio 0.5，达到阈值且 sample 仍远低于 MemTableMaxSamples
	points := []model.Point{
		{Measurement: "cpu", Timestamp: 10, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "cpu", Timestamp: 20, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
		{Measurement: "cpu", Timestamp: 5, Fields: map[string]model.FieldValue{"v": model.Float64Value(3)}},
		{Measurement: "cpu", Timestamp: 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(4)}},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{Sync: true}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write() error = %v", err)
	}
	shard := onlyShardForTest(t, eng)
	if shard.mem.SampleCount() != 0 {
		_ = eng.Close(ctx)
		t.Fatalf("mem samples = %d, want 0 after disorder flush", shard.mem.SampleCount())
	}
	if len(shard.manifest.Parts) == 0 {
		_ = eng.Close(ctx)
		t.Fatal("manifest parts empty, want flush produced part")
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestShouldFlushMemTableRespectsMinSamples(t *testing.T) {
	s := &Shard{
		opts: ShardOptions{
			MemTableMaxSamples:              100000,
			MemTableDisorderFlushRatio:      0.1,
			MemTableDisorderFlushMinSamples: 100,
		},
		mem: memTableStore{inner: mustNewMemWithOOO(t, 2, 1)},
	}
	if s.shouldFlushMemTable() {
		t.Fatal("shouldFlush = true below min samples, want false")
	}
}
