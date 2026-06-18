package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
)

func TestNextCompactionPlanTriggersByPartLimit(t *testing.T) {
	opts := model.CompactionOptions{
		MaxOutputPartBytes: 4096,
		Levels: []model.CompactionLevelOptions{
			{Level: 0, PartLimit: 2, MaxOutputPartBytes: 1024},
			{Level: 1, PartLimit: 2, MaxOutputPartBytes: 2048},
		},
	}
	plan, err := nextCompactionPlan([]sstable.PartMeta{
		{ID: "a", Level: 0, MinTime: 2},
		{ID: "b", Level: 0, MinTime: 1},
		{ID: "c", Level: 0, MinTime: 3},
	}, nil, opts)
	if err != nil {
		t.Fatalf("nextCompactionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("nextCompactionPlan() = nil, want plan")
	}
	if plan.level != 0 || plan.outputLevel != 1 {
		t.Fatalf("plan level=%d output=%d, want 0->1", plan.level, plan.outputLevel)
	}
	if plan.output.maxOutputPartBytes != 2048 {
		t.Fatalf("output max bytes = %d, want 2048", plan.output.maxOutputPartBytes)
	}
	if got := plan.candidates[0].ID; got != "b" {
		t.Fatalf("first candidate = %q, want sorted by time b", got)
	}
}

func TestNextCompactionPlanTriggersByLevelSize(t *testing.T) {
	dir := t.TempDir()
	first := writePlannerPartFile(t, dir, "a", "1234")
	second := writePlannerPartFile(t, dir, "b", "5678")
	opts := model.CompactionOptions{
		Levels: []model.CompactionLevelOptions{
			{Level: 1, PartLimit: 100, SizeLimit: 4},
			{Level: 2, PartLimit: 100, MaxOutputPartBytes: 8192},
		},
	}
	plan, err := nextCompactionPlan([]sstable.PartMeta{
		{ID: "a", Level: 1, Path: first},
		{ID: "b", Level: 1, Path: second},
	}, nil, opts)
	if err != nil {
		t.Fatalf("nextCompactionPlan(size) error = %v", err)
	}
	if plan == nil || plan.level != 1 || plan.outputLevel != 2 {
		t.Fatalf("plan = %#v, want level 1 to 2", plan)
	}
	if plan.output.maxOutputPartBytes != 8192 {
		t.Fatalf("output max bytes = %d, want 8192", plan.output.maxOutputPartBytes)
	}
}

func TestNextCompactionPlanReturnsNilBelowThreshold(t *testing.T) {
	plan, err := nextCompactionPlan([]sstable.PartMeta{
		{ID: "a", Level: 0},
		{ID: "b", Level: 0},
	}, nil, model.CompactionOptions{
		Levels: []model.CompactionLevelOptions{{Level: 0, PartLimit: 2}},
	})
	if err != nil {
		t.Fatalf("nextCompactionPlan() error = %v", err)
	}
	if plan != nil {
		t.Fatalf("nextCompactionPlan() = %#v, want nil", plan)
	}
}

func TestFullCompactionPlanUsesNextHighestLevel(t *testing.T) {
	plan := fullCompactionPlan([]sstable.PartMeta{
		{ID: "a", Level: 1},
		{ID: "b", Level: 3},
	}, nil, model.CompactionOptions{MaxOutputPartBytes: 1024})
	if plan == nil {
		t.Fatal("fullCompactionPlan() = nil, want plan")
	}
	if plan.outputLevel != 4 {
		t.Fatalf("output level = %d, want 4", plan.outputLevel)
	}
	if plan.output.maxOutputPartBytes != 1024 {
		t.Fatalf("output max bytes = %d, want 1024", plan.output.maxOutputPartBytes)
	}
}

func TestOutputOptionsForLevelUsesLevelCompression(t *testing.T) {
	opts := model.CompactionOptions{
		MaxOutputPartBytes: 1024,
		Levels: []model.CompactionLevelOptions{
			{
				Level:              1,
				MaxOutputPartBytes: 2048,
				Compression: model.CompressionOptions{
					Enabled:       true,
					Algorithm:     "zstd",
					MinPageValues: 1,
				},
			},
		},
	}
	output := outputOptionsForLevel(opts, 1)
	if output.maxOutputPartBytes != 2048 {
		t.Fatalf("max output bytes = %d, want 2048", output.maxOutputPartBytes)
	}
	if !output.compression.Enabled || output.compression.Algorithm != "zstd" {
		t.Fatalf("compression = %#v, want zstd", output.compression)
	}
}

func TestNextCompactionPlanReportsReasonScoreAndEstimates(t *testing.T) {
	dir := t.TempDir()
	first := writePlannerPartFile(t, dir, "a", "1234")
	second := writePlannerPartFile(t, dir, "b", "5678")
	opts := model.CompactionOptions{
		Levels: []model.CompactionLevelOptions{
			{Level: 0, PartLimit: 1, MaxOutputPartBytes: 1024},
			{Level: 1, PartLimit: 4, MaxOutputPartBytes: 2048},
		},
	}
	plan, err := nextCompactionPlan([]sstable.PartMeta{
		{ID: "a", Level: 0, Path: first, RowsCount: 10, MinSeriesID: 1, MaxSeriesID: 1, MinTime: 1, MaxTime: 10},
		{ID: "b", Level: 0, Path: second, RowsCount: 10, MinSeriesID: 2, MaxSeriesID: 2, MinTime: 1, MaxTime: 10},
	}, nil, opts)
	if err != nil {
		t.Fatalf("nextCompactionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("nextCompactionPlan() = nil, want plan")
	}
	if plan.reason != compactionReasonPartLimit {
		t.Fatalf("reason = %q, want %q", plan.reason, compactionReasonPartLimit)
	}
	if plan.score <= 1 {
		t.Fatalf("score = %f, want greater than 1", plan.score)
	}
	if plan.inputBytes != 8 || plan.outputEstimateBytes != 8 {
		t.Fatalf("bytes input=%d output=%d, want 8/8", plan.inputBytes, plan.outputEstimateBytes)
	}
	if plan.candidateSignature == "" {
		t.Fatal("candidate signature is empty")
	}
}

func TestNextCompactionPlanPrioritizesReadAmplificationLevel(t *testing.T) {
	dir := t.TempDir()
	l0a := writePlannerPartFile(t, dir, "l0a", "1")
	l0b := writePlannerPartFile(t, dir, "l0b", "2")
	l1a := writePlannerPartFile(t, dir, "l1a", "3333")
	l1b := writePlannerPartFile(t, dir, "l1b", "4444")
	opts := model.CompactionOptions{
		ReadAmplificationPartLimit: 1,
		Levels: []model.CompactionLevelOptions{
			{Level: 0, PartLimit: 1},
			{Level: 1, PartLimit: 100},
		},
	}
	plan, err := nextCompactionPlan([]sstable.PartMeta{
		{ID: "l0a", Level: 0, Path: l0a, RowsCount: 1, MinSeriesID: 1, MaxSeriesID: 1, MinTime: 1, MaxTime: 1},
		{ID: "l0b", Level: 0, Path: l0b, RowsCount: 1, MinSeriesID: 2, MaxSeriesID: 2, MinTime: 1, MaxTime: 1},
		{ID: "l1a", Level: 1, Path: l1a, RowsCount: 1, MinSeriesID: 1, MaxSeriesID: 3, MinTime: 1, MaxTime: 10},
		{ID: "l1b", Level: 1, Path: l1b, RowsCount: 1, MinSeriesID: 2, MaxSeriesID: 4, MinTime: 2, MaxTime: 11},
	}, nil, opts)
	if err != nil {
		t.Fatalf("nextCompactionPlan() error = %v", err)
	}
	if plan == nil {
		t.Fatal("nextCompactionPlan() = nil, want plan")
	}
	if plan.level != 1 || plan.reason != compactionReasonReadAmplification {
		t.Fatalf("plan level=%d reason=%q, want L1 read amplification", plan.level, plan.reason)
	}
}

func TestCompactionBacklogSnapshotCountsLevelsAndOverlaps(t *testing.T) {
	opts := model.CompactionOptions{
		BacklogDegradedThreshold: 2,
		Levels: []model.CompactionLevelOptions{
			{Level: 0, PartLimit: 1},
			{Level: 1, PartLimit: 4},
		},
	}
	snapshot, err := buildCompactionBacklog([]sstable.PartMeta{
		{ID: "l0a", Level: 0, BlockCount: 2, RowsCount: 1, MinSeriesID: 1, MaxSeriesID: 1, MinTime: 1, MaxTime: 1},
		{ID: "l0b", Level: 0, BlockCount: 2, RowsCount: 1, MinSeriesID: 2, MaxSeriesID: 2, MinTime: 1, MaxTime: 1},
		{ID: "l1a", Level: 1, BlockCount: 3, RowsCount: 1, MinSeriesID: 1, MaxSeriesID: 3, MinTime: 1, MaxTime: 10},
		{ID: "l1b", Level: 1, BlockCount: 3, RowsCount: 1, MinSeriesID: 2, MaxSeriesID: 4, MinTime: 2, MaxTime: 11},
	}, nil, opts)
	if err != nil {
		t.Fatalf("buildCompactionBacklog() error = %v", err)
	}
	if snapshot.PendingPlans != 2 || snapshot.OverlapCount != 1 || !snapshot.Degraded {
		t.Fatalf("snapshot = %#v, want two pending plans, one overlap, degraded", snapshot)
	}
	if snapshot.Levels[1].Reason != compactionReasonOverlap || snapshot.Levels[1].Score <= 1 {
		t.Fatalf("level 1 backlog = %#v, want overlap reason and score", snapshot.Levels[1])
	}
}

func writePlannerPartFile(t *testing.T, root string, name string, data string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte(data), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return dir
}
