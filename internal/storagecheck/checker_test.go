package storagecheck

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/sstable"
	"codeberg.org/mts/mts/internal/wal"
)

func TestCheckReportsOrphanPartMissingReferencedPartAndChecksum(t *testing.T) {
	dir := t.TempDir()
	good, err := sstable.WritePart(dir, 0, "sst-good", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart(good) error = %v", err)
	}
	orphan, err := sstable.WritePart(dir, 0, "sst-orphan", []model.ColumnData{checkColumn(2, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart(orphan) error = %v", err)
	}
	bad, err := sstable.WritePart(dir, 0, "sst-bad", []model.ColumnData{checkColumn(3, 1, 300)})
	if err != nil {
		t.Fatalf("WritePart(bad) error = %v", err)
	}
	corruptFile(t, filepath.Join(bad.Path, "values.bin"), 16)
	missing := sstable.PartMeta{ID: "sst-missing", Level: 0, Path: filepath.Join(dir, "sst-missing")}
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 1, Parts: []sstable.PartMeta{good, bad, missing}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	report, err := Check(dir, Options{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	assertIssue(t, report, SeverityWarn, orphan.Path, "orphan part")
	assertIssue(t, report, SeverityFatal, missing.Path, "manifest references missing part")
	assertIssue(t, report, SeverityFatal, bad.Path, "open part failed")
}

func TestCheckUnknownFilePolicies(t *testing.T) {
	dir := t.TempDir()
	unknown := filepath.Join(dir, "unknown.bin")
	if err := os.WriteFile(unknown, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(unknown) error = %v", err)
	}
	ignored, err := Check(dir, Options{UnknownFiles: UnknownFileIgnore})
	if err != nil {
		t.Fatalf("Check(ignore) error = %v", err)
	}
	if len(ignored.Issues) != 0 {
		t.Fatalf("ignore issues = %#v, want none", ignored.Issues)
	}
	warned, err := Check(dir, Options{UnknownFiles: UnknownFileWarn})
	if err != nil {
		t.Fatalf("Check(warn) error = %v", err)
	}
	assertIssue(t, warned, SeverityWarn, unknown, "unknown file")
	fatal, err := Check(dir, Options{UnknownFiles: UnknownFileFatal})
	if err != nil {
		t.Fatalf("Check(fatal) error = %v", err)
	}
	assertIssue(t, fatal, SeverityFatal, unknown, "unknown file")
}

func TestCheckReportsCorruptWALSegment(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{Sync: true})
	if err != nil {
		t.Fatalf("wal.Open() error = %v", err)
	}
	if err := log.Append([]model.ResolvedPoint{{SeriesID: 1, Timestamp: 1, WriteSeq: 1}}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	path := filepath.Join(dir, "000001.wal")
	corruptFile(t, path, 0)
	report, err := Check(dir, Options{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	assertIssue(t, report, SeverityFatal, path, "wal segment format error")
}

func checkColumn(seriesID uint64, fieldID uint32, count int) model.ColumnData {
	samples := make([]model.VersionedSample, count)
	for index := range samples {
		samples[index] = model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		}
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}
}

func corruptFile(t *testing.T, path string, offset int64) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", path, err)
	}
	if _, err := file.WriteAt([]byte{0xff}, offset); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt(%s) error = %v close = %v", path, err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%s) error = %v", path, err)
	}
}

func assertIssue(t *testing.T, report Report, severity Severity, path string, reason string) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Severity == severity && issue.Path == path && issue.Reason == reason {
			return
		}
	}
	t.Fatalf("issue severity=%s path=%s reason=%q not found in %#v", severity, path, reason, report.Issues)
}
