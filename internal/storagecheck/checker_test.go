package storagecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
	"github.com/openmts/mts/internal/wal"
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

func TestCheckDoesNotTreatCatalogMetadataAsPart(t *testing.T) {
	dir := t.TempDir()
	catalogDir := filepath.Join(dir, "catalog")
	if err := os.MkdirAll(catalogDir, 0700); err != nil {
		t.Fatalf("MkdirAll(catalog) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "metadata.bin"), []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile(catalog metadata) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "catalog.wal"), []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile(catalog wal) error = %v", err)
	}
	report, err := Check(dir, Options{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	for _, issue := range report.Issues {
		if issue.Path == catalogDir {
			t.Fatalf("catalog dir issue = %#v, want ignored as non-part", issue)
		}
	}
}

func TestCheckTreatsPartWithMissingComponentAsPart(t *testing.T) {
	dir := t.TempDir()
	part, err := sstable.WritePart(dir, 0, "sst-missing-values", []model.ColumnData{checkColumn(1, 1, 4)})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := os.Remove(filepath.Join(part.Path, "values.bin")); err != nil {
		t.Fatalf("Remove(values) error = %v", err)
	}
	report, err := Check(dir, Options{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	assertIssue(t, report, SeverityWarn, part.Path, "orphan part")
	assertIssue(t, report, SeverityFatal, part.Path, "open part failed")
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

func TestCheckReportsWALSegmentFormatVariants(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "truncated", data: []byte("short")},
		{name: "magic", data: append([]byte("BADSIG2"), make([]byte, walSegmentHeaderLen-len("BADSIG2"))...)},
		{name: "checksum", data: append([]byte(walSegmentMagic), make([]byte, walSegmentHeaderLen-len(walSegmentMagic))...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "000001.wal")
			if err := os.WriteFile(path, tt.data, 0600); err != nil {
				t.Fatalf("WriteFile(wal) error = %v", err)
			}
			report, err := Check(dir, Options{})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			assertIssue(t, report, SeverityFatal, path, "wal segment format error")
		})
	}
}

func TestCheckReportsManifestFormatAndNonDirectoryPart(t *testing.T) {
	badManifestDir := t.TempDir()
	badManifest := filepath.Join(badManifestDir, "MANIFEST.bin")
	if err := os.WriteFile(badManifest, []byte("bad"), 0600); err != nil {
		t.Fatalf("WriteFile(bad manifest) error = %v", err)
	}
	report, err := Check(badManifestDir, Options{})
	if err != nil {
		t.Fatalf("Check(bad manifest) error = %v", err)
	}
	assertIssue(t, report, SeverityFatal, badManifest, "manifest checksum or format error")

	dir := t.TempDir()
	partPath := filepath.Join(dir, "sst-file")
	if err := os.WriteFile(partPath, []byte("not a dir"), 0600); err != nil {
		t.Fatalf("WriteFile(part file) error = %v", err)
	}
	meta := sstable.PartMeta{ID: "sst-file", Path: partPath}
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 1, Parts: []sstable.PartMeta{meta}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	report, err = Check(dir, Options{})
	if err != nil {
		t.Fatalf("Check(non-dir part) error = %v", err)
	}
	assertIssue(t, report, SeverityFatal, partPath, "manifest references non-directory part")
}

func TestCheckerHelperBranches(t *testing.T) {
	if offset, kind := extractBlockLocation("read block offset=123 failed"); offset != 123 || kind != "block" {
		t.Fatalf("extractBlockLocation(block) = %d %q, want 123 block", offset, kind)
	}
	for _, message := range []string{"value page", "value index", "time block", "index"} {
		if _, kind := extractBlockLocation(message); kind == "" {
			t.Fatalf("extractBlockLocation(%q) kind is empty", message)
		}
	}
	for _, name := range []string{"000001.wal", "abc001.wal", "000001.bad"} {
		_ = isStorageWALSegmentName(name)
	}
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
