package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
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
