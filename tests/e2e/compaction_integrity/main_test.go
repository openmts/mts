package main

import (
	"testing"

	mts "codeberg.org/mts/mts"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestAssertCompactedRowsRejectsMismatch(t *testing.T) {
	if err := assertCompactedRows(nil); err == nil {
		t.Fatal("assertCompactedRows(empty) error = nil, want error")
	}
	rows := []mts.Row{{Fields: map[string]mts.FieldValue{"v": mts.Float64Value(2)}}}
	if err := assertCompactedRows(rows); err == nil {
		t.Fatal("assertCompactedRows(wrong value) error = nil, want error")
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
