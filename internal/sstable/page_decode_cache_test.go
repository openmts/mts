package sstable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestPageDecodeCacheRoundTripAndEvict(t *testing.T) {
	cache := newPageDecodeCache(2)
	key1 := pageDecodeKey{offset: 1, size: 2, start: 3, end: 4}
	key2 := pageDecodeKey{offset: 5, size: 6, start: 7, end: 8}
	key3 := pageDecodeKey{offset: 9, size: 10, start: 11, end: 12}
	samples := []model.VersionedSample{{Timestamp: 1, Value: model.Float64Value(1.5)}}
	cache.put(key1, samples)
	got, ok := cache.get(key1)
	if !ok || len(got) != 1 || got[0].Timestamp != 1 {
		t.Fatalf("get key1 = %#v ok=%v", got, ok)
	}
	// 修改返回副本不应污染缓存
	got[0].Timestamp = 99
	got2, ok := cache.get(key1)
	if !ok || got2[0].Timestamp != 1 {
		t.Fatalf("cache mutated by caller: %#v", got2)
	}
	cache.put(key2, samples)
	cache.put(key3, samples)
	if _, ok := cache.get(key1); ok {
		t.Fatal("key1 should be evicted")
	}
	if _, ok := cache.get(key2); !ok {
		t.Fatal("key2 missing")
	}
	if _, ok := cache.get(key3); !ok {
		t.Fatal("key3 missing")
	}
}

func TestPageDecodeCacheSkipsLargePages(t *testing.T) {
	cache := newPageDecodeCache(8)
	key := pageDecodeKey{offset: 1, size: 2, start: 3, end: 4}
	large := make([]model.VersionedSample, defaultPageDecodeCacheMaxSamples+1)
	for i := range large {
		large[i] = model.VersionedSample{Timestamp: int64(i), Value: model.Float64Value(1)}
	}
	cache.put(key, large)
	if _, ok := cache.get(key); ok {
		t.Fatal("large page should not be cached")
	}
	small := large[:defaultPageDecodeCacheMaxSamples]
	cache.put(key, small)
	if _, ok := cache.get(key); !ok {
		t.Fatal("boundary-size page should be cached")
	}
}

func TestConfigurePageDecodeCacheAndDisable(t *testing.T) {
	ConfigurePageDecodeCache(-1, 0)
	if cache := newPageDecodeCache(0); cache != nil {
		t.Fatal("limit -1 should disable default cache")
	}
	ConfigurePageDecodeCache(4, 8)
	cache := newPageDecodeCache(0)
	if cache == nil || cache.limit != 4 || cache.maxSamples != 8 {
		t.Fatalf("cache=%#v", cache)
	}
	// restore defaults for other tests
	ConfigurePageDecodeCache(0, 0)
}

func TestOpenPartTrustedDisablesPageCache(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{{
		SeriesID: 1, FieldID: 2, FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{Timestamp: 1, Value: model.Float64Value(1)}},
	}}
	meta, err := WritePart(dir, 0, "p", columns)
	if err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	trusted, err := OpenPartTrusted(meta.Path)
	if err != nil {
		t.Fatalf("OpenPartTrusted: %v", err)
	}
	defer func() { _ = trusted.Close() }()
	if trusted.pageCache != nil {
		t.Fatal("trusted part should not enable page cache")
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart: %v", err)
	}
	defer func() { _ = part.Close() }()
	if part.pageCache == nil {
		t.Fatal("query open should enable page cache by default")
	}
}
