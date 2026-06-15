package wal

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestAppendRejectsUnencodablePayloadAndEmptyReplay(t *testing.T) {
	log, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	bad := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 1,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 2, Type: model.FieldType(99), Value: model.FieldValue{Type: model.FieldType(99)}},
		},
	}
	if err := log.Append([]model.ResolvedPoint{bad}, false); err == nil {
		t.Fatal("Append(unsupported field type) error = nil, want encode error")
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() error = %v", err)
	}
	points, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() empty error = %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("empty replay count = %d, want 0", len(points))
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestReplayRejectsUnsupportedTypeBadPayloadAndSmallLength(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{name: "unsupported type", frame: encodeFrame(99, []byte{batchVersion, 0})},
		{name: "bad payload", frame: encodeFrame(recordWriteBatch, []byte{batchVersion})},
		{name: "small length", frame: smallLengthFrame()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "000001.wal"), tt.frame, 0600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			log, err := Open(dir, Options{})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if _, err := log.Replay(); err == nil {
				t.Fatal("Replay() error = nil, want error")
			}
			if err := log.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}

func TestBinaryPayloadDecodeValidationErrors(t *testing.T) {
	if _, err := decodeBatch([]byte{batchVersion, 0, 1}); err == nil {
		t.Fatal("decodeBatch(trailing) error = nil, want error")
	}
	if _, err := decodeBatch([]byte{99}); err == nil {
		t.Fatal("decodeBatch(version) error = nil, want error")
	}

	reader := newBatchReader([]byte{2})
	if _, err := reader.bool("bad bool"); err == nil {
		t.Fatal("bool(invalid) error = nil, want error")
	}
	if err := newBatchReader([]byte{1}).done("wal test"); err == nil {
		t.Fatal("done(trailing) error = nil, want error")
	}
	if _, err := uint32Value("field id", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	if _, err := readValuePayload(model.FieldType(99), newBatchReader(nil)); err == nil {
		t.Fatal("readValuePayload(unknown) error = nil, want error")
	}
}

func TestDecodeBatchRejectsTruncatedPayloadPrefixes(t *testing.T) {
	payload, err := encodeBatch([]model.ResolvedPoint{{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a", "zone": "z"},
		SeriesID:        1,
		Timestamp:       10,
		WriteSeq:        2,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "f", Type: model.FieldFloat64, Value: model.Float64Value(1)},
			{FieldID: 2, FieldName: "i", Type: model.FieldInt64, Value: model.Int64Value(2)},
			{FieldID: 3, FieldName: "s", Type: model.FieldString, Value: model.StringValue("ok")},
			{FieldID: 4, FieldName: "b", Type: model.FieldBool, Value: model.BoolValue(false)},
		},
	}})
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	for size := 0; size < len(payload); size++ {
		if _, err := decodeBatch(payload[:size]); err == nil {
			t.Fatalf("decodeBatch(prefix %d) error = nil, want error", size)
		}
	}
}

func TestTruncateAllAfterCloseIsSafe(t *testing.T) {
	log, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() after close error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() final error = %v", err)
	}
}

func TestOpenIgnoresNonSegmentsAndReusesLastSegment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ignore"), 0600); err != nil {
		t.Fatalf("WriteFile(non-segment) error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatalf("Mkdir(non-segment dir) error = %v", err)
	}
	record := model.ResolvedPoint{SeriesID: 10, Timestamp: 20, WriteSeq: 30}
	if err := os.WriteFile(filepath.Join(dir, "000003.wal"), encodeFrame(recordWriteBatch, mustBatch(t, []model.ResolvedPoint{record})), 0600); err != nil {
		t.Fatalf("WriteFile(segment) error = %v", err)
	}

	log, err := Open(dir, Options{SegmentBytes: 4096})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	another := model.ResolvedPoint{SeriesID: 11, Timestamp: 21, WriteSeq: 31}
	if err := log.Append([]model.ResolvedPoint{another}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Replay() count = %d, want 2", len(got))
	}
	if got[0].SeriesID != record.SeriesID || got[1].SeriesID != another.SeriesID {
		t.Fatalf("Replay() = %#v, want records from existing and appended segment", got)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestReplayRejectsPartialNonLastSegment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001.wal"), []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile(partial) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000002.wal"), nil, 0600); err != nil {
		t.Fatalf("WriteFile(empty) error = %v", err)
	}
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay() error = nil, want partial non-last segment error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTruncateAllKeepsNonSegmentFiles(t *testing.T) {
	dir := t.TempDir()
	notePath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(notePath, []byte("keep"), 0600); err != nil {
		t.Fatalf("WriteFile(note) error = %v", err)
	}
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() error = %v", err)
	}
	got, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("ReadFile(note) error = %v", err)
	}
	if string(got) != "keep" {
		t.Fatalf("note data = %q, want %q", string(got), "keep")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestInternalPathErrors(t *testing.T) {
	log := &Log{dir: "bad\x00path", opts: Options{SegmentBytes: 1}}
	if err := log.openLastSegment(); err == nil {
		t.Fatal("openLastSegment(invalid) error = nil, want error")
	}
	if err := log.openSegment(1); err == nil {
		t.Fatal("openSegment(invalid) error = nil, want error")
	}
	if err := log.TruncateAll(); err == nil {
		t.Fatal("TruncateAll(invalid) error = nil, want error")
	}
	if _, err := listSegments("bad\x00path"); err == nil {
		t.Fatal("listSegments(invalid) error = nil, want error")
	}
	if _, err := replaySegment("bad\x00path", true); err == nil {
		t.Fatal("replaySegment(invalid) error = nil, want error")
	}
	if err := normalizeReadError(errors.New("boom")); err == nil {
		t.Fatal("normalizeReadError() error = nil, want wrapped error")
	}
}

func mustBatch(t *testing.T, records []model.ResolvedPoint) []byte {
	t.Helper()
	data, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	return data
}

func smallLengthFrame() []byte {
	frame := make([]byte, 4)
	binary.BigEndian.PutUint32(frame, 1)
	return frame
}
