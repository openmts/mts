package engine

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/sstable"
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

func TestCompactionStatsRecorderTracksPlanAndDroppedRows(t *testing.T) {
	var recorder compactionStatsRecorder
	plan := compactionPlan{
		level:               1,
		outputLevel:         2,
		candidates:          []sstable.PartMeta{{ID: "a", BlockCount: 10}, {ID: "b", BlockCount: 12}},
		reason:              compactionReasonReadAmplification,
		score:               1002,
		inputBytes:          22,
		outputEstimateBytes: 18,
		candidateSignature:  "level:1|a|b",
	}
	attempt := recorder.beginPlan(plan)
	attempt.finishWithRows([]sstable.PartMeta{{ID: "out", BlockCount: 7}}, 3, nil)
	stats := recorder.snapshot()
	if stats.LastReason != compactionReasonReadAmplification || stats.LastLevel != 1 || stats.LastOutputLevel != 2 {
		t.Fatalf("stats = %#v, want last plan metadata", stats)
	}
	if stats.InputBytes != 22 || stats.OutputBytes != 7 || stats.DroppedRows != 3 {
		t.Fatalf("bytes/dropped = %d/%d/%d, want 22/7/3", stats.InputBytes, stats.OutputBytes, stats.DroppedRows)
	}
	if stats.MaxScore != 1002 {
		t.Fatalf("MaxScore = %f, want 1002", stats.MaxScore)
	}
}

func TestCompactionTaskStatusSnapshotReportsLastTask(t *testing.T) {
	var recorder compactionStatsRecorder
	plan := compactionPlan{
		level:              0,
		outputLevel:        1,
		candidates:         []sstable.PartMeta{{ID: "in", BlockCount: 5}},
		reason:             compactionReasonPartLimit,
		score:              2,
		candidateSignature: "level:0|in",
	}
	attempt := recorder.beginPlan(plan)
	attempt.finishWithRows(nil, 0, errors.New("corrupt part"))
	stats := recorder.snapshot()
	if stats.LastTask.ID == "" || stats.LastTask.State != compactionTaskFailed {
		t.Fatalf("LastTask = %#v, want failed task with id", stats.LastTask)
	}
	if stats.LastTask.Reason != compactionReasonPartLimit || stats.LastTask.Error != "corrupt part" {
		t.Fatalf("LastTask = %#v, want reason and error", stats.LastTask)
	}
	if stats.LastTask.Duration <= 0 {
		t.Fatalf("LastTask.Duration = %s, want positive", stats.LastTask.Duration)
	}
}
