package faultinject

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Operation string

const (
	OpCreate Operation = "create"
	OpWrite  Operation = "write"
	OpSync   Operation = "sync"
	OpRename Operation = "rename"
	OpRemove Operation = "remove"
	OpWalk   Operation = "walk"
	OpStat   Operation = "stat"
)

type FS struct {
	mu       sync.Mutex
	failures map[Operation]error
	next     map[Operation][]error
}

func NewFS() *FS {
	return &FS{
		failures: make(map[Operation]error),
		next:     make(map[Operation][]error),
	}
}

func (fs *FS) Fail(op Operation, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.failures[op] = err
}

func (fs *FS) FailNext(op Operation, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.next[op] = append(fs.next[op], err)
}

func (fs *FS) Before(operation string) error {
	return fs.err(Operation(operation))
}

func (fs *FS) Create(path string) (*os.File, error) {
	if err := fs.err(OpCreate); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
}

func (fs *FS) Write(file *os.File, data []byte) (int, error) {
	if err := fs.err(OpWrite); err != nil {
		return 0, err
	}
	return file.Write(data)
}

func (fs *FS) Sync(file *os.File) error {
	if err := fs.err(OpSync); err != nil {
		return err
	}
	return file.Sync()
}

func (fs *FS) Rename(oldPath string, newPath string) error {
	if err := fs.err(OpRename); err != nil {
		return err
	}
	return os.Rename(filepath.Clean(oldPath), filepath.Clean(newPath))
}

func (fs *FS) RemoveAll(path string) error {
	if err := fs.err(OpRemove); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Clean(path))
}

func (fs *FS) Stat(path string) (os.FileInfo, error) {
	if err := fs.err(OpStat); err != nil {
		return nil, err
	}
	return os.Stat(filepath.Clean(path))
}

func (fs *FS) Walk(root string, fn filepath.WalkFunc) error {
	if err := fs.err(OpWalk); err != nil {
		return err
	}
	return filepath.Walk(filepath.Clean(root), fn)
}

func (fs *FS) err(op Operation) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.next[op]) > 0 {
		err := fs.next[op][0]
		copy(fs.next[op], fs.next[op][1:])
		fs.next[op] = fs.next[op][:len(fs.next[op])-1]
		if err != nil {
			return fmt.Errorf("fault %s: %w", op, err)
		}
		return nil
	}
	err := fs.failures[op]
	if err == nil {
		return nil
	}
	return fmt.Errorf("fault %s: %w", op, err)
}
