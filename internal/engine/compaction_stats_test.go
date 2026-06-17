package engine

import (
	"errors"
	"testing"

	"codeberg.org/mts/mts/internal/sstable"
)

func TestCompactionStatsRecorderSnapshotAndMerge(t *testing.T) {
	var recorder compactionStatsRecorder
	attempt := recorder.begin([]sstable.PartMeta{{ID: "in", BlockCount: 2}})
	attempt.finish([]sstable.PartMeta{{ID: "out", BlockCount: 1}}, nil)
	stats := recorder.snapshot()
	if stats.Active != 0 || stats.Total != 1 || stats.Success != 1 || stats.InputBytes != 2 || stats.OutputBytes != 1 {
		t.Fatalf("success stats = %#v, want completed success with bytes", stats)
	}

	failed := recorder.begin([]sstable.PartMeta{{ID: "bad", BlockCount: 3}})
	failed.finish(nil, errors.New("boom"))
	stats = recorder.snapshot()
	if stats.Failure != 1 || stats.LastError != "boom" {
		t.Fatalf("failure stats = %#v, want one failure with last error", stats)
	}

	merged := mergeCompactionStats(stats, CompactionStats{Total: 2, Success: 2, LastDuration: stats.LastDuration + 1})
	if merged.Total != stats.Total+2 || merged.Success != stats.Success+2 {
		t.Fatalf("merged stats = %#v, want totals combined", merged)
	}
}
