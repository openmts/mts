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

func TestRepairDryRunAndApplyRemovesOnlyOrphans(t *testing.T) {
	dir := t.TempDir()
	kept, err := sstable.WritePart(dir, 0, "sst-kept", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart(kept) error = %v", err)
	}
	orphan, err := sstable.WritePart(dir, 0, "sst-orphan", []model.ColumnData{checkColumn(2, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart(orphan) error = %v", err)
	}
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 1, Parts: []sstable.PartMeta{kept}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	dryRun, err := Repair(dir, RepairOptions{})
	if err != nil {
		t.Fatalf("Repair(dry-run) error = %v", err)
	}
	if len(dryRun.Actions) != 1 || dryRun.Actions[0].Path != orphan.Path || dryRun.Actions[0].Applied {
		t.Fatalf("dry-run actions = %#v, want one unapplied orphan removal", dryRun.Actions)
	}
	if _, err := os.Stat(orphan.Path); err != nil {
		t.Fatalf("orphan removed during dry-run: %v", err)
	}
	applied, err := Repair(dir, RepairOptions{Apply: true})
	if err != nil {
		t.Fatalf("Repair(apply) error = %v", err)
	}
	if len(applied.Actions) != 1 || !applied.Actions[0].Applied {
		t.Fatalf("apply actions = %#v, want applied orphan removal", applied.Actions)
	}
	if _, err := os.Stat(orphan.Path); !os.IsNotExist(err) {
		t.Fatalf("orphan stat after apply = %v, want not exist", err)
	}
	if _, err := os.Stat(kept.Path); err != nil {
		t.Fatalf("kept part stat after apply = %v", err)
	}
}

func TestMigrateCreatesCheckpointAndCanResume(t *testing.T) {
	dir := t.TempDir()
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 7}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	dryRun, err := Migrate(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate(dry-run) error = %v", err)
	}
	if dryRun.Applied || dryRun.CheckpointPath == "" || dryRun.BackupPath == "" {
		t.Fatalf("dry-run migration = %#v, want unapplied checkpoint and backup paths", dryRun)
	}
	applied, err := Migrate(dir, MigrateOptions{Apply: true})
	if err != nil {
		t.Fatalf("Migrate(apply) error = %v", err)
	}
	if !applied.Applied {
		t.Fatalf("migration applied = false, want true")
	}
	if _, err := os.Stat(applied.BackupPath); err != nil {
		t.Fatalf("backup stat = %v", err)
	}
	if _, err := os.Stat(applied.CheckpointPath); err != nil {
		t.Fatalf("checkpoint stat = %v", err)
	}
	resumed, err := Migrate(dir, MigrateOptions{Apply: true})
	if err != nil {
		t.Fatalf("Migrate(resume) error = %v", err)
	}
	if !resumed.Resumed {
		t.Fatalf("resumed = false, want true")
	}
}

func TestRepairDoesNotRemoveUnknownPaths(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.bin")
	if err := os.WriteFile(unknown, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(unknown) error = %v", err)
	}
	plan, err := Repair(dir, RepairOptions{Apply: true})
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}
	if len(plan.Actions) != 0 {
		t.Fatalf("repair actions = %#v, want none for unknown files", plan.Actions)
	}
	if _, err := os.Stat(unknown); err != nil {
		t.Fatalf("unknown stat after repair = %v", err)
	}
}

func TestRepairAndMigratePropagateStorageErrors(t *testing.T) {
	if _, err := Repair("bad\x00path", RepairOptions{}); err == nil {
		t.Fatal("Repair(bad path) error = nil, want error")
	}

	dir := t.TempDir()
	orphan, err := sstable.WritePart(dir, 0, "sst-orphan", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if orphan.Path == "" {
		t.Fatal("orphan path is empty")
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpRemove, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	_, err = Repair(dir, RepairOptions{Apply: true})
	restore()
	if err == nil {
		t.Fatal("Repair(remove fault) error = nil, want error")
	}

	missing := t.TempDir()
	if _, err := Migrate(missing, MigrateOptions{Apply: true}); err == nil {
		t.Fatal("Migrate(missing manifest) error = nil, want error")
	}

	healthy := t.TempDir()
	if err := sstable.WriteManifest(healthy, sstable.Manifest{Sequence: 1}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	fs = faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, os.ErrPermission)
	restore = storagefs.SetFaultController(fs)
	_, err = Migrate(healthy, MigrateOptions{Apply: true})
	restore()
	if err == nil {
		t.Fatal("Migrate(write fault) error = nil, want error")
	}
}
