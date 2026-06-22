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

func TestReaderCorruptAndOrphanScenarios(t *testing.T) {
	for name, scenario := range map[string]func(string) error{
		"reader":    runReaderCompactionScenario,
		"corrupt":   runCorruptCompactionScenario,
		"orphan":    runOrphanCleanupScenario,
		"tombstone": runTombstoneCompactionScenario,
	} {
		t.Run(name, func(t *testing.T) {
			if err := scenario(t.TempDir()); err != nil {
				t.Fatalf("%s scenario error = %v", name, err)
			}
		})
	}
}
