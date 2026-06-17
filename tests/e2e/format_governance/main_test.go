package main

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/storagecheck"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestHelpersRejectInvalidInputs(t *testing.T) {
	if _, err := findShardDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("findShardDir(missing) error = nil, want error")
	}
	if err := assertNoJSONStorage(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("assertNoJSONStorage(missing) error = nil, want error")
	}
	report := storagecheck.Report{Issues: []storagecheck.Issue{{
		Severity: storagecheck.SeverityWarn,
		Path:     "p",
		Reason:   "r",
	}}}
	if !hasIssue(report, storagecheck.SeverityWarn, "p", "r") {
		t.Fatal("hasIssue() = false, want true")
	}
	if hasIssue(report, storagecheck.SeverityFatal, "p", "r") {
		t.Fatal("hasIssue(wrong severity) = true, want false")
	}
}

func TestAssertNoJSONStorageRejectsMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.bin"), []byte(`{"bad":true}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := assertNoJSONStorage(dir); err == nil {
		t.Fatal("assertNoJSONStorage(marker) error = nil, want error")
	}
}
