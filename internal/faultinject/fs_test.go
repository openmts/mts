package faultinject

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
