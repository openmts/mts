package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/wal"
)

func TestShardWriteAndFlushErrors(t *testing.T) {
	point := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 1,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 2, FieldName: "v", Type: model.FieldBool, Value: model.BoolValue(true)},
		},
	}
	shard := &Shard{
		wal:  mustOpenWAL(t),
		mem:  memStoreForTest(),
		opts: ShardOptions{Dir: "bad\x00path", Start: 0, End: int64(time.Hour), MemTableMaxSamples: 1},
		deps: normalizeShardDeps(shardDeps{}),
	}
	if err := shard.Write(point, false); err == nil {
		t.Fatal("Shard.Write() error = nil, want invalid flush path error")
	}
	if err := shard.wal.Close(); err != nil {
		t.Fatalf("wal.Close() error = %v", err)
	}
}

func TestOpenShardManifestErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.bin"), []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(bad manifest) error = %v", err)
	}
	if _, _, err := OpenShard(ShardOptions{Dir: dir, Start: 0, End: 1}); err == nil {
		t.Fatal("OpenShard(bad manifest) error = nil, want error")
	}

	dir = t.TempDir()
	manifest := sstable.Manifest{
		Parts: []sstable.PartMeta{{ID: "missing", Path: filepath.Join(dir, "missing")}},
	}
	if err := sstable.WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if _, _, err := OpenShard(ShardOptions{Dir: dir, Start: 0, End: 1}); err == nil {
		t.Fatal("OpenShard(missing part) error = nil, want error")
	}
	shard := &Shard{deps: normalizeShardDeps(shardDeps{})}
	if err := shard.removeOldParts([]sstable.PartMeta{{ID: "bad", Path: "bad\x00path"}}); err == nil {
		t.Fatal("removeOldParts(invalid) error = nil, want error")
	}
}

func TestEngineOpenFlushAndCompactErrors(t *testing.T) {
	root := t.TempDir()
	badShardDir := filepath.Join(root, "data", defaultDatabase, defaultRetentionPolicy, "shards", "0")
	if err := os.MkdirAll(badShardDir, 0700); err != nil {
		t.Fatalf("MkdirAll(bad shard) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(badShardDir, "MANIFEST.bin"), []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(bad manifest) error = %v", err)
	}
	if _, err := Open(context.Background(), model.Options{Path: root}); err == nil {
		t.Fatal("Open(bad shard manifest) error = nil, want error")
	}

	point := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 1,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 2, FieldName: "v", Type: model.FieldBool, Value: model.BoolValue(true)},
		},
	}
	badFlush := &Shard{
		wal:  mustOpenWAL(t),
		mem:  memStoreForTest(),
		opts: ShardOptions{Dir: "bad\x00path", Start: 0, End: int64(time.Hour)},
		deps: normalizeShardDeps(shardDeps{}),
	}
	if err := badFlush.mem.Apply(point); err != nil {
		t.Fatalf("mem.Apply() error = %v", err)
	}
	eng := &Engine{shards: map[string]*Shard{"bad": badFlush}}
	if err := eng.Flush(context.Background()); err == nil {
		t.Fatal("Engine.Flush() error = nil, want shard flush error")
	}
	if err := badFlush.wal.Close(); err != nil {
		t.Fatalf("bad flush wal Close() error = %v", err)
	}

	badCompact := &Shard{
		wal:  mustOpenWAL(t),
		mem:  memStoreForTest(),
		opts: ShardOptions{Dir: "bad\x00path", Start: 0, End: int64(time.Hour)},
		deps: normalizeShardDeps(shardDeps{}),
	}
	if err := badCompact.mem.Apply(point); err != nil {
		t.Fatalf("mem.Apply() compact error = %v", err)
	}
	eng = &Engine{shards: map[string]*Shard{"bad": badCompact}}
	if err := eng.Compact(context.Background()); err == nil {
		t.Fatal("Engine.Compact() error = nil, want shard compact error")
	}
	if err := badCompact.wal.Close(); err != nil {
		t.Fatalf("bad compact wal Close() error = %v", err)
	}
}

func mustOpenWAL(t *testing.T) *wal.Log {
	t.Helper()
	log, err := wal.Open(t.TempDir(), wal.Options{})
	if err != nil {
		t.Fatalf("wal.Open() error = %v", err)
	}
	return log
}
