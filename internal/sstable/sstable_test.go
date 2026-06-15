package sstable_test

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

func TestPartWriteReadFieldPruneAndManifest(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(5)},
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(7)},
			},
		},
	}
	meta, err := sstable.WritePart(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if meta.ID != "sst-000001" {
		t.Fatalf("part ID = %q, want %q", meta.ID, "sst-000001")
	}

	manifest := sstable.Manifest{Parts: []sstable.PartMeta{meta}}
	if err := sstable.WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	loaded, err := sstable.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(loaded.Parts) != 1 {
		t.Fatalf("manifest part count = %d, want 1", len(loaded.Parts))
	}

	part, err := sstable.OpenPart(filepath.Join(dir, meta.ID))
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if part.Meta().ID != meta.ID {
		t.Fatalf("part Meta ID = %q, want %q", part.Meta().ID, meta.ID)
	}
	emptyRange, err := part.Query(sstable.Query{Start: 20, End: 0})
	if err != nil {
		t.Fatalf("empty range Query() error = %v", err)
	}
	if len(emptyRange) != 0 {
		t.Fatalf("empty range count = %d, want 0", len(emptyRange))
	}
	got, err := part.Query(sstable.Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       20,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("column count = %d, want 1", len(got))
	}
	if got[0].Samples[1].Value.Float64 != 7 {
		t.Fatalf("second value = %v, want 7", got[0].Samples[1].Value.Float64)
	}

	pruned, err := part.Query(sstable.Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{99: {}},
		Start:     0,
		End:       20,
	})
	if err != nil {
		t.Fatalf("pruned Query() error = %v", err)
	}
	if len(pruned) != 0 {
		t.Fatalf("pruned column count = %d, want 0", len(pruned))
	}
}

func TestPartRejectsEmptyColumnsAndMissingManifestIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, err := sstable.WritePart(dir, 0, "sst-empty", nil); err == nil {
		t.Fatal("WritePart() empty error = nil, want error")
	}
	manifest, err := sstable.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() missing error = %v", err)
	}
	if len(manifest.Parts) != 0 {
		t.Fatalf("missing manifest parts = %d, want 0", len(manifest.Parts))
	}
	if err := sstable.WriteManifest(dir, sstable.Manifest{}); err != nil {
		t.Fatalf("WriteManifest() empty error = %v", err)
	}
	manifest, err = sstable.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() empty error = %v", err)
	}
	if manifest.Parts == nil {
		t.Fatal("manifest Parts = nil, want initialized empty slice")
	}
}

func TestPartQueryDetectsValueBlockCorruption(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldInt64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Int64Value(5)},
			},
		},
	}
	meta, err := sstable.WritePart(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	valuePath := filepath.Join(dir, meta.ID, "values.bin")
	file, err := os.OpenFile(valuePath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := file.WriteAt([]byte{0xff}, 12); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt() error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	part, err := sstable.OpenPart(filepath.Join(dir, meta.ID))
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	_, err = part.Query(sstable.Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       20,
	})
	if err == nil {
		t.Fatal("Query() corruption error = nil, want error")
	}
}

func TestPartMultiSeriesQueryAllAndInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		{
			SeriesID:  2,
			FieldID:   3,
			FieldType: model.FieldBool,
			Samples: []model.VersionedSample{
				{Timestamp: 30, WriteSeq: 1, Value: model.BoolValue(true)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldString,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.StringValue("a")},
			},
		},
	}
	meta, err := sstable.WritePart(dir, 0, "sst-000002", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := sstable.OpenPart(filepath.Join(dir, meta.ID))
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	got, err := part.Query(sstable.Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("Query() all error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("all column count = %d, want 2", len(got))
	}
	if got[0].SeriesID != 1 {
		t.Fatalf("first series = %d, want 1", got[0].SeriesID)
	}
	if _, err := sstable.OpenPart(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("OpenPart(missing) error = nil, want error")
	}
	if _, err := sstable.LoadManifest(filepath.Join(dir, "\x00bad")); err == nil {
		t.Fatal("LoadManifest(invalid) error = nil, want error")
	}
	if err := sstable.WriteManifest("bad\x00path", sstable.Manifest{}); err == nil {
		t.Fatal("WriteManifest(invalid) error = nil, want error")
	}
	if _, err := sstable.WritePart("bad\x00path", 0, "sst-bad", columns); err == nil {
		t.Fatal("WritePart(invalid) error = nil, want error")
	}
	badPart := filepath.Join(dir, "bad-part")
	if err := os.Mkdir(badPart, 0700); err != nil {
		t.Fatalf("Mkdir(bad-part) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(badPart, "metadata.json"), []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(bad metadata) error = %v", err)
	}
	if _, err := sstable.OpenPart(badPart); err == nil {
		t.Fatal("OpenPart(bad metadata) error = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.json"), []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(bad manifest) error = %v", err)
	}
	if _, err := sstable.LoadManifest(dir); err == nil {
		t.Fatal("LoadManifest(bad json) error = nil, want error")
	}
}
