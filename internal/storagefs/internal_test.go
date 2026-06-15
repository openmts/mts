package storagefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMkdirAllFixesExistingDirectoryMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll() setup error = %v", err)
	}
	if err := MkdirAll(dir); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != DirMode {
		t.Fatalf("dir mode = %v, want %v", info.Mode().Perm(), DirMode)
	}
}

func TestWriteFileAtomicParentIsFileReturnsError(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "data")
	if err := os.WriteFile(parent, []byte("not a dir"), FileMode); err != nil {
		t.Fatalf("WriteFile() setup error = %v", err)
	}
	if err := WriteFileAtomic(filepath.Join(parent, "meta.json"), []byte("x")); err == nil {
		t.Fatal("WriteFileAtomic(parent file) error = nil, want error")
	}
}

func TestWriteAndCloseSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := writeAndClose(file, []byte("ok")); err != nil {
		t.Fatalf("writeAndClose() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("file data = %q, want %q", string(got), "ok")
	}
}

func TestWriteAndCloseClosedFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := writeAndClose(file, []byte("x")); err == nil {
		t.Fatal("writeAndClose() error = nil, want error")
	}
}

func TestWriteAndCloseSyncErrorReturnsError(t *testing.T) {
	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, FileMode)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", os.DevNull, err)
	}
	if err := writeAndClose(file, []byte("x")); err == nil {
		t.Fatal("writeAndClose(sync error) error = nil, want error")
	}
}

func TestMkdirAllInvalidPathReturnsError(t *testing.T) {
	if err := MkdirAll("bad\x00path"); err == nil {
		t.Fatal("MkdirAll(invalid) error = nil, want error")
	}
}
