package storagefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testFaultController struct {
	operation string
	err       error
}

func (c testFaultController) Before(operation string) error {
	if operation == c.operation {
		return c.err
	}
	return nil
}

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

func TestWrappedOperationsAndFaultController(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a")
	file, err := OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := Write(file, []byte("ok")); err != nil {
		closeErr := file.Close()
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := Sync(file); err != nil {
		closeErr := file.Close()
		t.Fatalf("Sync() error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := Stat(path); err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if data, err := ReadFile(path); err != nil || string(data) != "ok" {
		t.Fatalf("ReadFile() = %q, %v; want ok", string(data), err)
	}
	if entries, err := ReadDir(dir); err != nil || len(entries) == 0 {
		t.Fatalf("ReadDir() entries=%d error=%v", len(entries), err)
	}
	walked := 0
	if err := Walk(dir, func(string, os.FileInfo, error) error {
		walked++
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if walked == 0 {
		t.Fatal("Walk() visited 0 entries")
	}
	walked = 0
	if err := WalkDir(dir, func(string, os.DirEntry, error) error {
		walked++
		return nil
	}); err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
	if walked == 0 {
		t.Fatal("WalkDir() visited 0 entries")
	}
	next := filepath.Join(dir, "b")
	if err := Rename(path, next); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := Remove(next); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
}

func TestFaultControllerRejectsOperations(t *testing.T) {
	boom := errors.New("boom")
	for _, item := range []struct {
		name string
		op   string
		run  func(string) error
	}{
		{name: "open_file", op: OpCreate, run: func(path string) error {
			file, err := OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
			if file != nil {
				closeErr := file.Close()
				err = errors.Join(err, closeErr)
			}
			return err
		}},
		{name: "create_temp", op: OpCreate, run: func(path string) error {
			file, err := CreateTemp(filepath.Dir(path), "fault-*")
			if file != nil {
				closeErr := file.Close()
				err = errors.Join(err, closeErr)
			}
			return err
		}},
		{name: "write", op: OpWrite, run: func(path string) error {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
			if err != nil {
				return err
			}
			_, writeErr := Write(file, []byte("x"))
			closeErr := file.Close()
			return errors.Join(writeErr, closeErr)
		}},
		{name: "sync", op: OpSync, run: func(path string) error {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
			if err != nil {
				return err
			}
			syncErr := Sync(file)
			closeErr := file.Close()
			return errors.Join(syncErr, closeErr)
		}},
		{name: "rename", op: OpRename, run: func(path string) error { return Rename(path, path+".next") }},
		{name: "remove", op: OpRemove, run: func(path string) error { return Remove(path) }},
		{name: "stat", op: OpStat, run: func(path string) error {
			_, err := Stat(path)
			return err
		}},
		{name: "read_dir", op: OpWalk, run: func(path string) error {
			_, err := ReadDir(filepath.Dir(path))
			return err
		}},
		{name: "read_file", op: OpStat, run: func(path string) error {
			_, err := ReadFile(path)
			return err
		}},
		{name: "walk", op: OpWalk, run: func(path string) error {
			return Walk(filepath.Dir(path), func(string, os.FileInfo, error) error { return nil })
		}},
		{name: "walk_dir", op: OpWalk, run: func(path string) error {
			return WalkDir(filepath.Dir(path), func(string, os.DirEntry, error) error { return nil })
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "x")
			if err := os.WriteFile(path, []byte("x"), FileMode); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			restore := SetFaultController(testFaultController{operation: item.op, err: boom})
			err := item.run(path)
			restore()
			if !errors.Is(err, boom) {
				t.Fatalf("%s error = %v, want boom", item.name, err)
			}
		})
	}
}

func TestWriteFileAtomicFaultBranches(t *testing.T) {
	for _, item := range []struct {
		name  string
		setup func(*testing.T, *faultSequence)
	}{
		{name: "mkdir", setup: func(_ *testing.T, seq *faultSequence) {
			seq.fail(OpCreate, errors.New("mkdir failed"))
		}},
		{name: "create_temp", setup: func(_ *testing.T, seq *faultSequence) {
			seq.allow(OpCreate)
			seq.fail(OpCreate, errors.New("create temp failed"))
		}},
		{name: "write", setup: func(_ *testing.T, seq *faultSequence) {
			seq.fail(OpWrite, errors.New("write failed"))
		}},
		{name: "sync", setup: func(_ *testing.T, seq *faultSequence) {
			seq.fail(OpSync, errors.New("sync failed"))
		}},
		{name: "rename", setup: func(_ *testing.T, seq *faultSequence) {
			seq.fail(OpRename, errors.New("rename failed"))
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			seq := newFaultSequence()
			item.setup(t, seq)
			restore := SetFaultController(seq)
			err := WriteFileAtomic(filepath.Join(t.TempDir(), "file"), []byte("x"))
			restore()
			if err == nil {
				t.Fatal("WriteFileAtomic() error = nil, want injected error")
			}
		})
	}
}

func TestSyncDirFaultBranches(t *testing.T) {
	dir := t.TempDir()
	for _, item := range []struct {
		name string
		op   string
	}{
		{name: "open", op: OpStat},
		{name: "sync", op: OpSync},
	} {
		t.Run(item.name, func(t *testing.T) {
			restore := SetFaultController(testFaultController{operation: item.op, err: errors.New("sync dir fault")})
			err := SyncDir(dir)
			restore()
			if err == nil {
				t.Fatal("SyncDir() error = nil, want injected error")
			}
		})
	}
}

type faultSequence struct {
	ops map[string][]error
}

func newFaultSequence() *faultSequence {
	return &faultSequence{ops: make(map[string][]error)}
}

func (s *faultSequence) allow(operation string) {
	s.ops[operation] = append(s.ops[operation], nil)
}

func (s *faultSequence) fail(operation string, err error) {
	s.ops[operation] = append(s.ops[operation], err)
}

func (s *faultSequence) Before(operation string) error {
	queue := s.ops[operation]
	if len(queue) == 0 {
		return nil
	}
	err := queue[0]
	copy(queue, queue[1:])
	s.ops[operation] = queue[:len(queue)-1]
	return err
}
