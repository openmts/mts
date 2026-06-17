package sstable

import (
	"path/filepath"
	"testing"
)

func TestManifestPersistsSequenceAndRejectsRegression(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{Sequence: 2, Parts: []PartMeta{}}
	if err := WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if loaded.Sequence != manifest.Sequence {
		t.Fatalf("manifest sequence = %d, want %d", loaded.Sequence, manifest.Sequence)
	}
	if _, err := LoadManifestStrict(dir, 3); err == nil {
		t.Fatal("LoadManifestStrict(regression) error = nil, want error")
	}
}

func TestLoadManifestRejectsMissingReferencedPart(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		Sequence: 1,
		Parts: []PartMeta{{
			ID:    "sst-missing",
			Level: 1,
			Path:  filepath.Join(dir, "sst-missing"),
		}},
	}
	if err := WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if _, err := LoadManifestStrict(dir, 0); err == nil {
		t.Fatal("LoadManifestStrict(missing part) error = nil, want error")
	}
}
