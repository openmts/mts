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

func TestMainFunction(t *testing.T) {
	main()
}

func TestAssertRecoveredRowsRejectsMismatch(t *testing.T) {
	if err := assertRecoveredRows(nil, 1); err == nil {
		t.Fatal("assertRecoveredRows(empty) error = nil, want error")
	}
	rows := []mts.Row{{Timestamp: 1}}
	if err := assertRecoveredRows(rows, 1); err != nil {
		t.Fatalf("assertRecoveredRows(valid) error = %v", err)
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
