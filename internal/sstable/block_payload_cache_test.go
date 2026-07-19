package sstable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestBlockPayloadCacheRoundTripEvictAndDisable(t *testing.T) {
	ConfigureBlockPayloadCache(2, 1<<20)
	cache := newBlockPayloadCacheIfEnabled(true)
	if cache == nil {
		t.Fatal("cache nil")
	}
	key1 := blockPayloadKey{name: valuesFile, offset: 1, size: 10}
	key2 := blockPayloadKey{name: valuesFile, offset: 2, size: 10}
	key3 := blockPayloadKey{name: valuesFile, offset: 3, size: 10}
	cache.put(key1, []byte("abc"))
	got, ok := cache.get(key1)
	if !ok || string(got) != "abc" {
		t.Fatalf("get=%q ok=%v", got, ok)
	}
	got[0] = 'x'
	got2, _ := cache.get(key1)
	if string(got2) != "abc" {
		t.Fatalf("cache mutated: %q", got2)
	}
	cache.put(key2, []byte("def"))
	cache.put(key3, []byte("ghi"))
	if _, ok := cache.get(key1); ok {
		t.Fatal("key1 should be evicted")
	}
	ConfigureBlockPayloadCache(-1, 0)
	if cache := newBlockPayloadCacheIfEnabled(true); cache != nil {
		t.Fatal("disabled cache should be nil")
	}
	ConfigureBlockPayloadCache(0, 0) // restore defaults
}

func TestOpenPartTrustedDisablesBlockCache(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "b", []model.ColumnData{{
		SeriesID: 1, FieldID: 2, FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{Timestamp: 1, Value: model.Float64Value(1)}},
	}})
	if err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	trusted, err := OpenPartTrusted(meta.Path)
	if err != nil {
		t.Fatalf("OpenPartTrusted: %v", err)
	}
	defer func() { _ = trusted.Close() }()
	if trusted.blockCache != nil {
		t.Fatal("trusted should disable block cache")
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart: %v", err)
	}
	defer func() { _ = part.Close() }()
	if part.blockCache == nil {
		t.Fatal("query open should enable block cache")
	}
}

func TestBlockPayloadCacheSkipsIndexBlocks(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "idx-skip", []model.ColumnData{{
		SeriesID: 1, FieldID: 2, FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{Timestamp: 1, Value: model.Float64Value(1.5)}},
	}})
	if err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart: %v", err)
	}
	defer func() { _ = part.Close() }()
	rows, err := part.loadIndexRows()
	if err != nil || len(rows) == 0 {
		t.Fatalf("loadIndexRows: %v rows=%d", err, len(rows))
	}
	// 读取 timestamps 两次，第二次应命中 block cache（通过 stats 或可重复查询验证功能正确）。
	cols, err := part.QuerySeriesIDs(Query{Start: 0, End: 10}, []uint64{1})
	if err != nil {
		t.Fatalf("QuerySeriesIDs first: %v", err)
	}
	if len(cols) != 1 || len(cols[0].Samples) != 1 {
		t.Fatalf("cols=%#v", cols)
	}
	cols2, err := part.QuerySeriesIDs(Query{Start: 0, End: 10}, []uint64{1})
	if err != nil {
		t.Fatalf("QuerySeriesIDs second: %v", err)
	}
	if len(cols2) != 1 || cols2[0].Samples[0].Value.Float64 != 1.5 {
		t.Fatalf("cols2=%#v", cols2)
	}
	// index 不入缓存：破坏 index 后已打开 part 的 QuerySeriesIDs 应失败。
	size, err := PartLogicalComponentSize(meta.Path, indexFile)
	if err != nil {
		t.Fatalf("index size: %v", err)
	}
	corrupt := make([]byte, size)
	for i := range corrupt {
		corrupt[i] = 0xff
	}
	if err := OverwriteLogicalComponentAt(meta.Path, indexFile, 0, corrupt); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, err := part.QuerySeriesIDs(Query{Start: 0, End: 10}, []uint64{1}); err == nil {
		t.Fatal("expected corrupt index error")
	}
}
