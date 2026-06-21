package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRunSmoke(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestWithWriteFaultRejectsMissingFault(t *testing.T) {
	if err := withWriteFault(func() error { return nil }); err == nil {
		t.Fatal("withWriteFault(nil) error = nil, want error")
	}
	if err := withWriteFault(func() error { return errors.New("plain") }); err == nil {
		t.Fatal("withWriteFault(plain) error = nil, want error")
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir(filepath.Join("bad\x00path")); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
