package engine

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
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
