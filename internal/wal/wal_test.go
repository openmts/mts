package wal_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"codeberg.org/mts/mts/internal/model"
	"codeberg.org/mts/mts/internal/wal"
)

func TestWALAppendReplayAndTruncateTail(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{SegmentBytes: 96, Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	records := []model.ResolvedPoint{
		{
			SeriesID:  1,
			Timestamp: 10,
			WriteSeq:  7,
			Fields: []model.ResolvedField{
				{FieldID: 2, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(3)},
			},
		},
	}
	if err := log.Append(records, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	appendPartialRecord(t, dir)

	log, err = wal.Open(dir, wal.Options{SegmentBytes: 96})
	if err != nil {
		t.Fatalf("Open() after append error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if !reflect.DeepEqual(got, records) {
		t.Fatalf("Replay() = %#v, want %#v", got, records)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() after replay error = %v", err)
	}
}

func TestWALDetectsMiddleCorruption(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{SegmentBytes: 4096, Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{SeriesID: 1, Timestamp: 10, WriteSeq: 1}
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() second error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path := filepath.Join(dir, "000001.wal")
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := file.WriteAt([]byte{0xff}, 12); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt() error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close corrupt file error = %v", err)
	}

	log, err = wal.Open(dir, wal.Options{SegmentBytes: 4096})
	if err != nil {
		t.Fatalf("Open() corrupt error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay() error = nil, want corruption error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() corrupt error = %v", err)
	}
}

func TestWALRolloverBatchSyncAndTruncateAll(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{SegmentBytes: 80, BatchRecords: 2})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 10,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 2, FieldName: "v", Type: model.FieldBool, Value: model.BoolValue(true)},
		},
	}
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append() second error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("segment count = %d, want at least 2", len(entries))
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() after truncate error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("replayed after truncate = %d, want 0", len(got))
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
}

func TestWALReplayRejectsUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "000001.wal")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	frame := []byte{0, 0, 0, 6, 99, 1, 0, 0, 0, 0}
	if err := os.WriteFile(path, frame, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	log, err := wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay() error = nil, want unsupported version error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWALOpenInvalidPathReturnsError(t *testing.T) {
	if _, err := wal.Open("bad\x00path", wal.Options{}); err == nil {
		t.Fatal("Open(invalid) error = nil, want error")
	}
}

func appendPartialRecord(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "000001.wal")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("OpenFile() partial error = %v", err)
	}
	if _, err := file.Write([]byte{1, 2, 3}); err != nil {
		closeErr := file.Close()
		t.Fatalf("Write() partial error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() partial error = %v", err)
	}
}
