package main

import (
	"runtime"
	"testing"

	mts "github.com/openmts/mts"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunRejectsInvalidTempDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不支持 /dev/null 作为无效临时目录路径")
	}
	t.Setenv("TMPDIR", "/dev/null")
	if err := run(); err == nil {
		t.Fatal("run(invalid temp dir) error = nil, want error")
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
