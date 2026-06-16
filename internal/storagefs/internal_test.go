package storagefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndCloseSuccessAndClosedFile(t *testing.T) {
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
		t.Fatalf("file data = %q, want ok", string(got))
	}

	closed, err := os.OpenFile(filepath.Join(t.TempDir(), "closed"), os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		t.Fatalf("OpenFile(closed) error = %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := writeAndClose(closed, []byte("x")); err == nil {
		t.Fatal("writeAndClose(closed) error = nil, want error")
	}
}

func TestMkdirAllRejectsInvalidPath(t *testing.T) {
	if err := MkdirAll("bad\x00path"); err == nil {
		t.Fatal("MkdirAll(invalid) error = nil, want error")
	}
}
