package main

import (
	"testing"

	mts "github.com/openmts/mts"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunRejectsInvalidTempDir(t *testing.T) {
	t.Setenv("TMPDIR", "/dev/null")
	if err := run(); err == nil {
		t.Fatal("run(invalid temp dir) error = nil, want error")
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestAssertManifestRowsRejectsMismatch(t *testing.T) {
	if err := assertManifestRows(nil); err == nil {
		t.Fatal("assertManifestRows(empty) error = nil, want error")
	}
	rows := []mts.Row{{Fields: map[string]mts.FieldValue{"v": mts.Float64Value(99)}}}
	if err := assertManifestRows(rows); err == nil {
		t.Fatal("assertManifestRows(wrong value) error = nil, want error")
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
