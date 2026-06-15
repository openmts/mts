package storagefs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirMode  os.FileMode = 0700
	FileMode os.FileMode = 0600
)

func MkdirAll(path string) error {
	clean := filepath.Clean(path)
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

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, FileMode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("set temp file permissions: %w", err)
	}
	if err := os.Rename(tmpName, clean); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	if err := SyncDir(dir); err != nil {
		return err
	}
	return nil
}

func SyncDir(path string) error {
	dir, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	if err := dir.Sync(); err != nil {
		closeErr := dir.Close()
		return errors.Join(fmt.Errorf("sync directory: %w", err), closeErr)
	}
	if err := dir.Close(); err != nil {
		return fmt.Errorf("close directory: %w", err)
	}
	return nil
}

func RemoveAll(path string) error {
	if err := os.RemoveAll(filepath.Clean(path)); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	return nil
}

func writeAndClose(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write temp file: %w", err), closeErr)
	}
	if err := file.Sync(); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("sync temp file: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	return nil
}
