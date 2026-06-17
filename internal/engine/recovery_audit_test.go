package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
)

func TestOpenShardReturnsRecoveryFatalForMissingManifestPart(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "sst-000001")
	manifest := sstable.Manifest{Parts: []sstable.PartMeta{{
		ID:   "sst-000001",
		Path: missing,
	}}}
	if err := sstable.WriteManifest(dir, manifest); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	_, _, err := OpenShard(ShardOptions{Dir: dir, Start: 0, End: 100})
	if !errors.Is(err, ErrRecoveryFatal) {
		t.Fatalf("OpenShard() error = %v, want ErrRecoveryFatal", err)
	}
	var issue *RecoveryIssue
	if !errors.As(err, &issue) {
		t.Fatalf("OpenShard() error = %T, want *RecoveryIssue", err)
	}
	if issue.Kind != RecoveryIssueMissingPart {
		t.Fatalf("RecoveryIssue.Kind = %q, want %q", issue.Kind, RecoveryIssueMissingPart)
	}
	if issue.Path != missing {
		t.Fatalf("RecoveryIssue.Path = %q, want %q", issue.Path, missing)
	}
}

func TestOpenShardReturnsRecoveryFatalForManifestMetadataMismatch(t *testing.T) {
	dir := t.TempDir()
	meta, err := sstable.WritePart(dir, 0, "sst-000001", []model.ColumnData{columnForOrphanTest()})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	badMeta := meta
	badMeta.RowsCount++
	if err := sstable.WriteManifest(dir, sstable.Manifest{Parts: []sstable.PartMeta{badMeta}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	_, _, err = OpenShard(ShardOptions{Dir: dir, Start: 0, End: 100})
	if !errors.Is(err, ErrRecoveryFatal) {
		t.Fatalf("OpenShard() error = %v, want ErrRecoveryFatal", err)
	}
	var issue *RecoveryIssue
	if !errors.As(err, &issue) {
		t.Fatalf("OpenShard() error = %T, want *RecoveryIssue", err)
	}
	if issue.Kind != RecoveryIssueMetadataMismatch {
		t.Fatalf("RecoveryIssue.Kind = %q, want %q", issue.Kind, RecoveryIssueMetadataMismatch)
	}
	if !strings.Contains(issue.Error(), "rows_count") {
		t.Fatalf("RecoveryIssue.Error() = %q, want rows_count", issue.Error())
	}
}

func TestOpenShardRecordsMaintenanceIssueForRemovedOrphanPart(t *testing.T) {
	dir := t.TempDir()
	meta, err := sstable.WritePart(dir, 0, "sst-000001", []model.ColumnData{columnForOrphanTest()})
	if err != nil {
		t.Fatalf("WritePart(valid) error = %v", err)
	}
	if err := sstable.WriteManifest(dir, sstable.Manifest{Parts: []sstable.PartMeta{meta}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	orphan, err := sstable.WritePart(dir, 0, "sst-999999", []model.ColumnData{columnForOrphanTest()})
	if err != nil {
		t.Fatalf("WritePart(orphan) error = %v", err)
	}

	shard, _, err := OpenShard(ShardOptions{Dir: dir, Start: 0, End: 100})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	if _, err := os.Stat(orphan.Path); !errors.Is(err, os.ErrNotExist) {
		closeErr := shard.Close()
		t.Fatalf("orphan stat = %v, want not exist close = %v", err, closeErr)
	}
	var issue *RecoveryIssue
	if !errors.As(shard.maintenanceErr, &issue) {
		closeErr := shard.Close()
		t.Fatalf("maintenanceErr = %v, want RecoveryIssue close = %v", shard.maintenanceErr, closeErr)
	}
	if issue.Kind != RecoveryIssueOrphanPartRemoved {
		closeErr := shard.Close()
		t.Fatalf("RecoveryIssue.Kind = %q, want %q close = %v", issue.Kind, RecoveryIssueOrphanPartRemoved, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenShardRecordsMaintenanceIssueForRemovedTempManifestFile(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, ".tmp-manifest")
	if err := os.WriteFile(tempPath, []byte("partial"), 0600); err != nil {
		t.Fatalf("WriteFile(temp) error = %v", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("Chmod(dir) error = %v", err)
	}

	shard, _, err := OpenShard(ShardOptions{Dir: dir, Start: 0, End: 100})
	if err != nil {
		t.Fatalf("OpenShard() error = %v", err)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		closeErr := shard.Close()
		t.Fatalf("temp stat = %v, want not exist close = %v", err, closeErr)
	}
	var issue *RecoveryIssue
	if !errors.As(shard.maintenanceErr, &issue) {
		closeErr := shard.Close()
		t.Fatalf("maintenanceErr = %v, want RecoveryIssue close = %v", shard.maintenanceErr, closeErr)
	}
	if issue.Kind != RecoveryIssueTempRemoved {
		closeErr := shard.Close()
		t.Fatalf("RecoveryIssue.Kind = %q, want %q close = %v", issue.Kind, RecoveryIssueTempRemoved, closeErr)
	}
	if err := shard.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
