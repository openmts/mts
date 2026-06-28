package main

import (
	"os"
	"path/filepath"
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

func TestAssertNoJSONStorageRejectsMarkersAndMissingPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.bin"), []byte(`{"bad":true}`), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := assertNoJSONStorage(dir); err == nil {
		t.Fatal("assertNoJSONStorage(marker) error = nil, want error")
	}
	if err := assertNoJSONStorage(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("assertNoJSONStorage(missing) error = nil, want error")
	}
}

func TestAssertPrunedColumnsRejectsMismatch(t *testing.T) {
	if err := assertPrunedColumns(nil); err == nil {
		t.Fatal("assertPrunedColumns(empty) error = nil, want error")
	}
	columns := []mts.ColumnSeries{{Values: []mts.FieldValue{mts.Int64Value(41)}}}
	if err := assertPrunedColumns(columns); err == nil {
		t.Fatal("assertPrunedColumns(wrong value) error = nil, want error")
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
