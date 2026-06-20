package faultinject

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/openmts/mts/internal/storagefs"
)

func TestFSInjectsOperationFailure(t *testing.T) {
	fs := NewFS()
	fs.Fail(OpCreate, errors.New("disk full"))
	if _, err := fs.Create(t.TempDir() + "/x"); err == nil {
		t.Fatal("Create() error = nil, want injected error")
	}
}

func TestFSOperations(t *testing.T) {
	dir := t.TempDir()
	fs := NewFS()
	path := filepath.Join(dir, "a")
	file, err := fs.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := fs.Write(file, []byte("x")); err != nil {
		closeErr := file.Close()
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := fs.Sync(file); err != nil {
		closeErr := file.Close()
		t.Fatalf("Sync() error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	next := filepath.Join(dir, "b")
	if err := fs.Rename(path, next); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if _, err := fs.Stat(next); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	visited := 0
	err = fs.Walk(dir, func(string, os.FileInfo, error) error {
		visited++
		return nil
	})
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if visited == 0 {
		t.Fatal("Walk() visited = 0, want entries")
	}
	if err := fs.RemoveAll(next); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
}

func TestFSShortWriteNext(t *testing.T) {
	fs := NewFS()
	path := filepath.Join(t.TempDir(), "short")
	file, err := fs.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	fs.ShortWriteNext(OpWrite, 2)
	written, err := fs.Write(file, []byte("abcdef"))
	closeErr := file.Close()
	if err != nil {
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if written != 2 {
		t.Fatalf("Write() written = %d, want 2", written)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ab" {
		t.Fatalf("file data = %q, want ab", string(got))
	}
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
}

func TestFSBeforeWriteAndWritePrefixBoundaries(t *testing.T) {
	fs := NewFS()
	file, err := os.CreateTemp(t.TempDir(), "before-write-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	written, handled, err := fs.BeforeWrite(file, []byte("abcdef"))
	if err != nil || handled || written != 0 {
		t.Fatalf("BeforeWrite(no short) = written %d handled %v err %v, want zero false nil", written, handled, err)
	}
	fs.ShortWriteNext(OpWrite, 99)
	written, handled, err = fs.BeforeWrite(file, []byte("abcdef"))
	if err != nil || !handled || written != 6 {
		t.Fatalf("BeforeWrite(short cap) = written %d handled %v err %v, want 6 true nil", written, handled, err)
	}
	fs.ShortWriteNext(OpWrite, -1)
	written, handled, err = fs.BeforeWrite(file, []byte("abcdef"))
	if err != nil || !handled || written != 0 {
		t.Fatalf("BeforeWrite(negative) = written %d handled %v err %v, want 0 true nil", written, handled, err)
	}
	if _, err := writePrefix(nil, []byte("x"), 1); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("writePrefix(nil) error = %v, want invalid", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFSFailNextENOSPCPropagatesThroughStorageFS(t *testing.T) {
	fs := NewFS()
	fs.FailNext(OpWrite, syscall.ENOSPC)
	path := filepath.Join(t.TempDir(), "segment.wal")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, storagefs.FileMode)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	restore := storagefs.SetFaultController(fs)
	err = storagefs.WriteFull(file, []byte("record"))
	restore()
	closeErr := file.Close()
	if !storagefs.IsNoSpace(err) {
		t.Fatalf("WriteFull() error = %v, want ENOSPC close = %v", err, closeErr)
	}
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
}

func TestFSInjectsEveryOperationFailure(t *testing.T) {
	for _, op := range []Operation{OpWrite, OpSync, OpRename, OpRemove, OpStat, OpWalk} {
		t.Run(string(op), func(t *testing.T) {
			fs := NewFS()
			fs.Fail(op, errors.New("boom"))
			file, err := os.CreateTemp(t.TempDir(), "fault-*")
			if err != nil {
				t.Fatalf("CreateTemp() error = %v", err)
			}
			switch op {
			case OpWrite:
				_, err = fs.Write(file, []byte("x"))
			case OpSync:
				err = fs.Sync(file)
			case OpRename:
				err = fs.Rename(file.Name(), file.Name()+"-next")
			case OpRemove:
				err = fs.RemoveAll(file.Name())
			case OpStat:
				_, err = fs.Stat(file.Name())
			case OpWalk:
				err = fs.Walk(t.TempDir(), func(string, os.FileInfo, error) error { return nil })
			}
			closeErr := file.Close()
			if err == nil {
				t.Fatalf("%s error = nil, want injected error close = %v", op, closeErr)
			}
			if closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})
	}
}
