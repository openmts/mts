package storagefs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/storagefs"
)

func TestSecureDirsAndAtomicFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data")
	if err := storagefs.MkdirAll(dir); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if info.Mode().Perm() != fs.FileMode(0700) {
		t.Fatalf("dir mode = %v, want %v", info.Mode().Perm(), fs.FileMode(0700))
	}

	path := filepath.Join(dir, "meta.json")
	if err := storagefs.WriteFileAtomic(path, []byte("ok")); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("file data = %q, want %q", string(got), "ok")
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if info.Mode().Perm() != fs.FileMode(0600) {
		t.Fatalf("file mode = %v, want %v", info.Mode().Perm(), fs.FileMode(0600))
	}
	if err := storagefs.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Stat() after RemoveAll error = %v, want not exist", err)
	}
}

func TestSyncDirMissingPathReturnsError(t *testing.T) {
	err := storagefs.SyncDir(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("SyncDir() error = nil, want error")
	}
	if err := storagefs.WriteFileAtomic("bad\x00path", []byte("x")); err == nil {
		t.Fatal("WriteFileAtomic(invalid) error = nil, want error")
	}
	if err := storagefs.RemoveAll("bad\x00path"); err == nil {
		t.Fatal("RemoveAll(invalid) error = nil, want error")
	}
	targetDir := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatalf("Mkdir(target) error = %v", err)
	}
	if err := storagefs.WriteFileAtomic(targetDir, []byte("x")); err == nil {
		t.Fatal("WriteFileAtomic(directory target) error = nil, want error")
	}
}
