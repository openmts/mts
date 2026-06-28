package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	mts "github.com/openmts/mts"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestRunRejectsInvalidTempDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不支持 /dev/null 作为无效临时目录路径")
	}
	t.Setenv("TMPDIR", "/dev/null")
	if err := run(); err == nil {
		t.Fatal("run(invalid temp dir) error = nil, want error")
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid path) error = nil, want error")
	}
}

func TestOpenPublicEngineRejectsInvalidExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 使用 ACL 进行访问控制，跳过 Unix 权限检查测试")
	}
	root := filepath.Join(t.TempDir(), "mts")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatalf("Mkdir(root) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "marker"), []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}
	eng, err := openPublicEngine(t.Context(), root)
	if err == nil {
		closeErr := eng.Close(t.Context())
		t.Fatalf("openPublicEngine(unsafe dir) error = nil, want error close=%v", closeErr)
	}
}

func TestAssertFieldSchemasRejectsMissingField(t *testing.T) {
	fields := []mts.FieldSchema{{Name: "usage", Type: mts.FieldFloat64}}
	if err := assertFieldSchemas(fields); err == nil {
		t.Fatal("assertFieldSchemas(missing fields) error = nil, want error")
	}
}

func TestAssertRowsRejectsIteratorErrors(t *testing.T) {
	iter := scriptedRowIterator{err: errors.New("boom")}
	if err := assertRows(&iter); err == nil {
		t.Fatal("assertRows(iterator error) error = nil, want error")
	}
}

func TestAssertUsageColumnRejectsWrongValues(t *testing.T) {
	iter := scriptedColumnIterator{
		columns: []mts.ColumnSeries{{
			FieldName: "usage",
			Values:    []mts.FieldValue{mts.Float64Value(0.1)},
		}},
	}
	if err := assertUsageColumn(&iter); err == nil {
		t.Fatal("assertUsageColumn(wrong values) error = nil, want error")
	}
}

type scriptedRowIterator struct {
	rows   []mts.Row
	index  int
	err    error
	closed bool
}

func (i *scriptedRowIterator) Next() bool {
	if i.index >= len(i.rows) {
		return false
	}
	i.index++
	return true
}

func (i *scriptedRowIterator) Row() mts.Row {
	return i.rows[i.index-1]
}

func (i *scriptedRowIterator) Err() error {
	return i.err
}

func (i *scriptedRowIterator) Close() error {
	i.closed = true
	return nil
}

type scriptedColumnIterator struct {
	columns []mts.ColumnSeries
	index   int
	err     error
	closed  bool
}

func (i *scriptedColumnIterator) Next() bool {
	if i.index >= len(i.columns) {
		return false
	}
	i.index++
	return true
}

func (i *scriptedColumnIterator) Column() mts.ColumnSeries {
	return i.columns[i.index-1]
}

func (i *scriptedColumnIterator) Err() error {
	return i.err
}

func (i *scriptedColumnIterator) Close() error {
	i.closed = true
	return nil
}
