package sstable

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/faultinject"
	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/storagefs"
)

func TestWritePartFailureRemovesPartialPartOnWriteError(t *testing.T) {
	dir := t.TempDir()
	partPath := filepath.Join(dir, "sst-partial-write")
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, errors.New("write failed"))
	restore := storagefs.SetFaultController(fs)
	_, err := WritePart(dir, 0, filepath.Base(partPath), atomicWriteColumnForTest())
	restore()
	if err == nil {
		t.Fatal("WritePart() error = nil, want injected write error")
	}
	assertPartPathRemoved(t, partPath)
}

func TestWritePartFailureRemovesPartialPartOnEncodeError(t *testing.T) {
	dir := t.TempDir()
	partPath := filepath.Join(dir, "sst-partial-encode")
	columns := atomicWriteColumnForTest()
	columns[0].FieldType = model.FieldType(255)

	_, err := WritePart(dir, 0, filepath.Base(partPath), columns)
	if err == nil {
		t.Fatal("WritePart() error = nil, want encode error")
	}
	assertPartPathRemoved(t, partPath)
}

func atomicWriteColumnForTest() []model.ColumnData {
	return []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 1, WriteSeq: 1, Value: model.Float64Value(1)},
		},
	}}
}

func assertPartPathRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("part path stat = %v, want not exist", err)
	}
}
