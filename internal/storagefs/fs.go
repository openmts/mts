package storagefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	DirMode  os.FileMode = 0700
	FileMode os.FileMode = 0600
)

const (
	OpCreate = "create"
	OpWrite  = "write"
	OpSync   = "sync"
	OpRename = "rename"
	OpRemove = "remove"
	OpStat   = "stat"
	OpWalk   = "walk"
)

type FaultController interface {
	Before(operation string) error
}

var faultState struct {
	mu         sync.RWMutex
	controller FaultController
}

func SetFaultController(controller FaultController) func() {
	faultState.mu.Lock()
	previous := faultState.controller
	faultState.controller = controller
	faultState.mu.Unlock()
	return func() {
		faultState.mu.Lock()
		faultState.controller = previous
		faultState.mu.Unlock()
	}
}

func MkdirAll(path string) error {
	clean := filepath.Clean(path)
	if err := before(OpCreate); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.MkdirAll(clean, DirMode); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.Chmod(clean, DirMode); err != nil {
		return fmt.Errorf("set directory permissions: %w", err)
	}
	return nil
}

func WriteFileAtomic(path string, data []byte) error {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	if err := MkdirAll(dir); err != nil {
		return err
	}

	tmp, err := CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, FileMode); err != nil {
		_ = Remove(tmpName)
		return fmt.Errorf("set temp file permissions: %w", err)
	}
	if err := Rename(tmpName, clean); err != nil {
		_ = Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	if err := SyncDir(dir); err != nil {
		return err
	}
	return nil
}

func SyncDir(path string) error {
	dir, err := Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	if err := Sync(dir); err != nil {
		closeErr := dir.Close()
		return errors.Join(fmt.Errorf("sync directory: %w", err), closeErr)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close directory: %w", err)
	}
	return nil
}

func RemoveAll(path string) error {
	if err := before(OpRemove); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	if err := os.RemoveAll(filepath.Clean(path)); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	return nil
}

func Open(path string) (*os.File, error) {
	if err := before(OpStat); err != nil {
		return nil, err
	}
	return os.Open(filepath.Clean(path))
}

func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&os.O_CREATE != 0 {
		if err := before(OpCreate); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(filepath.Clean(path), flag, perm)
}

func CreateTemp(dir string, pattern string) (*os.File, error) {
	if err := before(OpCreate); err != nil {
		return nil, err
	}
	return os.CreateTemp(filepath.Clean(dir), pattern)
}

func Write(file *os.File, data []byte) (int, error) {
	if err := before(OpWrite); err != nil {
		return 0, err
	}
	return file.Write(data)
}

func Sync(file *os.File) error {
	if err := before(OpSync); err != nil {
		return err
	}
	return file.Sync()
}

func Rename(oldPath string, newPath string) error {
	if err := before(OpRename); err != nil {
		return err
	}
	return os.Rename(filepath.Clean(oldPath), filepath.Clean(newPath))
}

func Remove(path string) error {
	if err := before(OpRemove); err != nil {
		return err
	}
	return os.Remove(filepath.Clean(path))
}

func Stat(path string) (os.FileInfo, error) {
	if err := before(OpStat); err != nil {
		return nil, err
	}
	return os.Stat(filepath.Clean(path))
}

func ReadDir(path string) ([]os.DirEntry, error) {
	if err := before(OpWalk); err != nil {
		return nil, err
	}
	return os.ReadDir(filepath.Clean(path))
}

func ReadFile(path string) ([]byte, error) {
	if err := before(OpStat); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Clean(path))
}

func Walk(root string, fn filepath.WalkFunc) error {
	if err := before(OpWalk); err != nil {
		return err
	}
	return filepath.Walk(filepath.Clean(root), fn)
}

func WalkDir(root string, fn fs.WalkDirFunc) error {
	if err := before(OpWalk); err != nil {
		return err
	}
	return filepath.WalkDir(filepath.Clean(root), fn)
}

func writeAndClose(file *os.File, data []byte) error {
	if _, err := Write(file, data); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write temp file: %w", err), closeErr)
	}
	if err := Sync(file); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("sync temp file: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return nil
}

func before(operation string) error {
	faultState.mu.RLock()
	controller := faultState.controller
	faultState.mu.RUnlock()
	if controller == nil {
		return nil
	}
	return controller.Before(operation)
}
