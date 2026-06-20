package storagecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/storagefs"
)

func TestSnapshotRestoreCopiesHealthyTree(t *testing.T) {
	source := t.TempDir()
	part, err := sstable.WritePart(source, 0, "sst-source", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := sstable.WriteManifest(source, sstable.Manifest{Sequence: 1, Parts: []sstable.PartMeta{part}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	result, err := Snapshot(source, target, SnapshotOptions{})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if result.Files == 0 || result.Bytes == 0 {
		t.Fatalf("snapshot result = %#v, want files and bytes", result)
	}
	report, err := Check(target, Options{})
	if err != nil {
		t.Fatalf("Check(snapshot) error = %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("snapshot issues = %#v, want none", report.Issues)
	}

	restore := filepath.Join(t.TempDir(), "restore")
	restored, err := Restore(target, restore, SnapshotOptions{})
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Files != result.Files || restored.Bytes == 0 {
		t.Fatalf("restore result = %#v, want same file count and non-zero bytes as %#v", restored, result)
	}
	if _, err := os.Stat(filepath.Join(restore, "MANIFEST.bin")); err != nil {
		t.Fatalf("restored manifest stat error = %v", err)
	}
}

func TestSnapshotRejectsFatalSourceAndExistingTarget(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "000001.wal"), []byte("bad"), 0600); err != nil {
		t.Fatalf("WriteFile(bad wal) error = %v", err)
	}
	if _, err := Snapshot(source, filepath.Join(t.TempDir(), "snapshot"), SnapshotOptions{}); err == nil {
		t.Fatal("Snapshot(fatal source) error = nil, want error")
	}

	healthy := t.TempDir()
	if err := sstable.WriteManifest(healthy, sstable.Manifest{Sequence: 1}); err != nil {
		t.Fatalf("WriteManifest(healthy) error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	if _, err := Snapshot(healthy, target, SnapshotOptions{}); err == nil {
		t.Fatal("Snapshot(existing target) error = nil, want error")
	}
	if _, err := Snapshot(healthy, target, SnapshotOptions{Overwrite: true}); err != nil {
		t.Fatalf("Snapshot(overwrite) error = %v", err)
	}
}

func TestSnapshotHelperErrorPaths(t *testing.T) {
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpStat, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	err := ensureTargetAvailable(filepath.Join(t.TempDir(), "target"), false)
	restore()
	if err == nil {
		t.Fatal("ensureTargetAvailable(stat fault) error = nil, want error")
	}

	if err := copyFile(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "out"), &SnapshotResult{}); err == nil {
		t.Fatal("copyFile(missing) error = nil, want error")
	}

	dir := t.TempDir()
	part := sstable.PartMeta{ID: "sst-1", Path: filepath.Join(t.TempDir(), "outside")}
	got := relocatedPartPath(dir, part, t.TempDir(), filepath.Join(t.TempDir(), "new"))
	if got != filepath.Join(dir, "sst-1") {
		t.Fatalf("relocatedPartPath(outside) = %q, want manifest-relative fallback", got)
	}
	empty := relocatedPartPath(dir, sstable.PartMeta{ID: "sst-2"}, t.TempDir(), filepath.Join(t.TempDir(), "new"))
	if empty != filepath.Join(dir, "sst-2") {
		t.Fatalf("relocatedPartPath(empty) = %q, want manifest-relative fallback", empty)
	}
}

func TestSnapshotPropagatesPublishFaults(t *testing.T) {
	source := t.TempDir()
	if err := sstable.WriteManifest(source, sstable.Manifest{Sequence: 1}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpCreate, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	_, err := Snapshot(source, filepath.Join(t.TempDir(), "snapshot"), SnapshotOptions{})
	restore()
	if err == nil {
		t.Fatal("Snapshot(create temp fault) error = nil, want error")
	}

	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	fs = faultinject.NewFS()
	fs.FailNext(faultinject.OpRemove, os.ErrPermission)
	restore = storagefs.SetFaultController(fs)
	_, err = Snapshot(source, target, SnapshotOptions{Overwrite: true})
	restore()
	if err == nil {
		t.Fatal("Snapshot(remove target fault) error = nil, want error")
	}

	part, err := sstable.WritePart(source, 0, "sst-rewrite", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart(rewrite) error = %v", err)
	}
	if err := sstable.WriteManifest(source, sstable.Manifest{Sequence: 2, Parts: []sstable.PartMeta{part}}); err != nil {
		t.Fatalf("WriteManifest(rewrite) error = %v", err)
	}
	fs = faultinject.NewFS()
	fs.FailNext(faultinject.OpRename, os.ErrPermission)
	restore = storagefs.SetFaultController(fs)
	_, err = Snapshot(source, filepath.Join(t.TempDir(), "rewrite"), SnapshotOptions{})
	restore()
	if err == nil {
		t.Fatal("Snapshot(manifest rewrite fault) error = nil, want error")
	}
}

func TestRestoreAndCopyFileErrorBranches(t *testing.T) {
	source := t.TempDir()
	if err := sstable.WriteManifest(source, sstable.Manifest{Sequence: 1}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("MkdirAll(target) error = %v", err)
	}
	if _, err := Restore(source, target, SnapshotOptions{}); err == nil {
		t.Fatal("Restore(existing target) error = nil, want error")
	}

	input := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(input, []byte("payload"), 0600); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	err := copyFile(input, filepath.Join(t.TempDir(), "out"), &SnapshotResult{})
	restore()
	if err == nil {
		t.Fatal("copyFile(write fault) error = nil, want error")
	}
}
