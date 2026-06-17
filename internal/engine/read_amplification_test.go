package engine

import (
	"testing"

	"codeberg.org/mts/mts/internal/sstable"
)

func TestComputeLevelHealthDetectsOverlaps(t *testing.T) {
	health := ComputeLevelHealth([]sstable.PartMeta{
		{ID: "l1-a", Level: 1, MinTime: 0, MaxTime: 10, MinSeriesID: 1, MaxSeriesID: 5, RowsCount: 1},
		{ID: "l1-b", Level: 1, MinTime: 5, MaxTime: 20, MinSeriesID: 3, MaxSeriesID: 7, RowsCount: 1},
		{ID: "l2-a", Level: 2, MinTime: 0, MaxTime: 10, MinSeriesID: 10, MaxSeriesID: 11, RowsCount: 1},
	})
	if len(health.Levels) != 2 {
		t.Fatalf("level count = %d, want 2", len(health.Levels))
	}
	levelOne := health.Levels[1]
	if levelOne.PartCount != 2 || levelOne.OverlapCount != 1 {
		t.Fatalf("level one health = %#v, want two parts and one overlap", levelOne)
	}
	if !health.Degraded {
		t.Fatal("health Degraded = false, want true")
	}
}

func TestLevelHealthExposesOverlapAndScore(t *testing.T) {
	health := ComputeLevelHealth([]sstable.PartMeta{
		{ID: "a", Level: 2, MinTime: 0, MaxTime: 10, MinSeriesID: 1, MaxSeriesID: 5, RowsCount: 1, BlockCount: 2},
		{ID: "b", Level: 2, MinTime: 5, MaxTime: 20, MinSeriesID: 3, MaxSeriesID: 7, RowsCount: 1, BlockCount: 3},
	})
	level := health.Levels[2]
	if level.Bytes != 5 || level.OverlapCount != 1 || level.Score <= 1 {
		t.Fatalf("level health = %#v, want bytes, overlap and score", level)
	}
}

func TestEngineCompactionStatsSnapshotAggregatesShards(t *testing.T) {
	engine := &Engine{shards: map[string]*Shard{
		"a": {},
		"b": {},
	}}
	first := engine.shards["a"].compactionStats.begin(nil)
	first.finish(nil, nil)
	second := engine.shards["b"].compactionStats.begin(nil)
	second.finish(nil, nil)
	stats := engine.CompactionStatsSnapshot()
	if stats.Total != 2 || stats.Success != 2 {
		t.Fatalf("CompactionStatsSnapshot() = %#v, want two successes", stats)
	}
	if got := engine.shards["a"].CompactionStatsSnapshot(); got.Total != 1 {
		t.Fatalf("Shard CompactionStatsSnapshot() = %#v, want one total", got)
	}
}
