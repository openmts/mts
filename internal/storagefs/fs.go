package storagefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
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

var ErrShortWrite = errors.New("short write")

type OpError struct {
	Operation string
	Path      string
	Err       error
}

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Operation, e.Path, e.Err)
}

func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type ShortWriteError struct {
	Written  int
	Expected int
}

func (e *ShortWriteError) Error() string {
	if e == nil {
		return ErrShortWrite.Error()
	}
	return fmt.Sprintf("%s: wrote %d of %d bytes", ErrShortWrite, e.Written, e.Expected)
}

func (e *ShortWriteError) Is(target error) bool {
	return target == ErrShortWrite
}

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
		return wrapOp(OpCreate, clean, err)
	}
	if err := os.MkdirAll(clean, DirMode); err != nil {
		return wrapOp(OpCreate, clean, err)
	}
	if err := os.Chmod(clean, DirMode); err != nil {
		return wrapOp(OpCreate, clean, err)
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
	// Windows 的 NTFS 不支持对目录句柄调用 fsync，跳过即可
	if runtime.GOOS == "windows" {
		return nil
	}
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
	clean := filepath.Clean(path)
	if err := before(OpRemove); err != nil {
		return wrapOp(OpRemove, clean, err)
	}
	if err := os.RemoveAll(clean); err != nil {
		return wrapOp(OpRemove, clean, err)
	}
	return nil
}

func Open(path string) (*os.File, error) {
	clean := filepath.Clean(path)
	if err := before(OpStat); err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	file, err := os.Open(clean)
	if err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	return file, nil
}

func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	clean := filepath.Clean(path)
	if flag&os.O_CREATE != 0 {
		if err := before(OpCreate); err != nil {
			return nil, wrapOp(OpCreate, clean, err)
		}
	}
	file, err := os.OpenFile(clean, flag, perm)
	if err != nil {
		return nil, wrapOp(openOperation(flag), clean, err)
	}
	return file, nil
}

func CreateTemp(dir string, pattern string) (*os.File, error) {
	clean := filepath.Clean(dir)
	if err := before(OpCreate); err != nil {
		return nil, wrapOp(OpCreate, clean, err)
	}
	file, err := os.CreateTemp(clean, pattern)
	if err != nil {
		return nil, wrapOp(OpCreate, clean, err)
	}
	return file, nil
}

func Write(file *os.File, data []byte) (int, error) {
	path := filePath(file)
	if file == nil {
		return 0, wrapOp(OpWrite, path, fs.ErrInvalid)
	}
	if err := before(OpWrite); err != nil {
		return 0, wrapOp(OpWrite, path, err)
	}
	if written, handled, err := beforeWrite(file, data); handled {
		if err != nil {
			return written, wrapOp(OpWrite, path, err)
		}
		return written, nil
	}
	written, err := file.Write(data)
	if err != nil {
		return written, wrapOp(OpWrite, path, err)
	}
	return written, nil
}

func WriteFull(file *os.File, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	written, err := Write(file, data)
	if err != nil {
		return writeFullError(filePath(file), written, len(data), err)
	}
	if written != len(data) {
		return wrapOp(OpWrite, filePath(file), newShortWriteError(written, len(data)))
	}
	return nil
}

func Sync(file *os.File) error {
	path := filePath(file)
	if file == nil {
		return wrapOp(OpSync, path, fs.ErrInvalid)
	}
	if err := before(OpSync); err != nil {
		return wrapOp(OpSync, path, err)
	}
	if err := file.Sync(); err != nil {
		return wrapOp(OpSync, path, err)
	}
	return nil
}

func Rename(oldPath string, newPath string) error {
	oldClean := filepath.Clean(oldPath)
	newClean := filepath.Clean(newPath)
	if err := before(OpRename); err != nil {
		return wrapOp(OpRename, oldClean, err)
	}
	if err := os.Rename(oldClean, newClean); err != nil {
		return wrapOp(OpRename, oldClean, err)
	}
	return nil
}

func Remove(path string) error {
	clean := filepath.Clean(path)
	if err := before(OpRemove); err != nil {
		return wrapOp(OpRemove, clean, err)
	}
	if err := os.Remove(clean); err != nil {
		return wrapOp(OpRemove, clean, err)
	}
	return nil
}

func Stat(path string) (os.FileInfo, error) {
	clean := filepath.Clean(path)
	if err := before(OpStat); err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	return info, nil
}

func ValidateStrictPermissions(path string) error {
	// Windows 使用 ACL 进行访问控制，Unix 权限位无实际意义，跳过检查
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := Stat(path)
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	limit := FileMode
	kind := "file"
	if info.IsDir() {
		limit = DirMode
		kind = "directory"
	}
	if perm&^limit != 0 {
		return fmt.Errorf("%s permissions too wide: got %04o want no wider than %04o", kind, perm, limit)
	}
	return nil
}

func ReadDir(path string) ([]os.DirEntry, error) {
	clean := filepath.Clean(path)
	if err := before(OpWalk); err != nil {
		return nil, wrapOp(OpWalk, clean, err)
	}
	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, wrapOp(OpWalk, clean, err)
	}
	return entries, nil
}

func ReadFile(path string) ([]byte, error) {
	clean := filepath.Clean(path)
	if err := before(OpStat); err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return nil, wrapOp(OpStat, clean, err)
	}
	return data, nil
}

func Walk(root string, fn filepath.WalkFunc) error {
	clean := filepath.Clean(root)
	if err := before(OpWalk); err != nil {
		return wrapOp(OpWalk, clean, err)
	}
	if err := filepath.Walk(clean, fn); err != nil {
		return wrapOp(OpWalk, clean, err)
	}
	return nil
}

func WalkDir(root string, fn fs.WalkDirFunc) error {
	clean := filepath.Clean(root)
	if err := before(OpWalk); err != nil {
		return wrapOp(OpWalk, clean, err)
	}
	if err := filepath.WalkDir(clean, fn); err != nil {
		return wrapOp(OpWalk, clean, err)
	}
	return nil
}

func writeAndClose(file *os.File, data []byte) error {
	if err := WriteFull(file, data); err != nil {
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

func IsNoSpace(err error) bool {
	return errors.Is(err, syscall.ENOSPC)
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

func beforeWrite(file *os.File, data []byte) (int, bool, error) {
	faultState.mu.RLock()
	controller := faultState.controller
	faultState.mu.RUnlock()
	writer, ok := controller.(interface {
		BeforeWrite(*os.File, []byte) (int, bool, error)
	})
	if !ok {
		return 0, false, nil
	}
	return writer.BeforeWrite(file, data)
}

func openOperation(flag int) string {
	if flag&os.O_CREATE != 0 {
		return OpCreate
	}
	return OpStat
}

func filePath(file *os.File) string {
	if file == nil {
		return ""
	}
	return file.Name()
}

func wrapOp(operation string, path string, err error) error {
	if err == nil {
		return nil
	}
	var opErr *OpError
	if errors.As(err, &opErr) {
		return err
	}
	return &OpError{Operation: operation, Path: path, Err: err}
}

func writeFullError(path string, written int, expected int, err error) error {
	if written == expected {
		return err
	}
	return &OpError{
		Operation: OpWrite,
		Path:      path,
		Err:       errors.Join(err, newShortWriteError(written, expected)),
	}
}

func newShortWriteError(written int, expected int) error {
	return &ShortWriteError{Written: written, Expected: expected}
}
