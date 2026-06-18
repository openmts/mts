package wal_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
	"github.com/openmts/mts/internal/wal"
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

func TestWALPayloadIsBinary(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		SeriesID:        1,
		Timestamp:       10,
		WriteSeq:        2,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "value", Type: model.FieldFloat64, Value: model.Float64Value(1.5)},
			{FieldID: 2, FieldName: "state", Type: model.FieldString, Value: model.StringValue("ok")},
		},
	}
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "000001.wal"))
	if err != nil {
		t.Fatalf("ReadFile(wal) error = %v", err)
	}
	if bytes.Contains(data, []byte(`"series_id"`)) || bytes.Contains(data, []byte(`{"`)) {
		t.Fatal("wal payload contains JSON marker")
	}
	log, err = wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open(replay) error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if !reflect.DeepEqual(got, []model.ResolvedPoint{record}) {
		t.Fatalf("Replay() = %#v, want %#v", got, []model.ResolvedPoint{record})
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close(replay) error = %v", err)
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
	log, err := wal.Open(dir, wal.Options{SegmentBytes: 40, BatchRecords: 2})
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

func TestWALCheckpointRemoveFailurePreservesReplay(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := walRecordForFaultTest()
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpRemove, errors.New("remove failed"))
	restore := storagefs.SetFaultController(fs)
	err = log.Checkpoint()
	restore()
	if err == nil {
		closeErr := log.Close()
		t.Fatalf("Checkpoint() error = nil, want remove failure close = %v", closeErr)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertWALReplaysRecords(t, dir, []model.ResolvedPoint{record})
}

func TestWALCheckpointSyncFailurePreservesReplay(t *testing.T) {
	dir := t.TempDir()
	log, err := wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := walRecordForFaultTest()
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpSync, errors.New("sync failed"))
	restore := storagefs.SetFaultController(fs)
	err = log.Checkpoint()
	restore()
	if err == nil {
		closeErr := log.Close()
		t.Fatalf("Checkpoint() error = nil, want sync failure close = %v", closeErr)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertWALReplaysRecords(t, dir, []model.ResolvedPoint{record})
}

func TestWALReplayRejectsUnsupportedRecordType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "000001.wal")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	frame := unsupportedRecordTypeFrame()
	if err := os.WriteFile(path, frame, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	log, err := wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay() error = nil, want unsupported record type error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func unsupportedRecordTypeFrame() []byte {
	body := []byte{99}
	sum := crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli))
	frame := make([]byte, 4, 9)
	binary.BigEndian.PutUint32(frame, uint32(len(body)+4))
	frame = append(frame, body...)
	return binary.BigEndian.AppendUint32(frame, sum)
}

func walRecordForFaultTest() model.ResolvedPoint {
	return model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 10,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "v", Type: model.FieldFloat64, Value: model.Float64Value(1)},
		},
	}
}

func assertWALReplaysRecords(t *testing.T, dir string, want []model.ResolvedPoint) {
	t.Helper()
	log, err := wal.Open(dir, wal.Options{})
	if err != nil {
		t.Fatalf("Open(replay) error = %v", err)
	}
	got, replayErr := log.Replay()
	closeErr := log.Close()
	if replayErr != nil {
		t.Fatalf("Replay() error = %v close = %v", replayErr, closeErr)
	}
	if closeErr != nil {
		t.Fatalf("Close(replay) error = %v", closeErr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Replay() = %#v, want %#v", got, want)
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
