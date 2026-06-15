package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
)

func TestDecodeLineRejectsBadCRCAndApplyEntryIgnoresUnknown(t *testing.T) {
	payload, err := encodeWALEntry(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu"},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry() error = %v", err)
	}
	frame := codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, payload)
	frame[len(frame)-1] ^= 0xff
	if _, err := decodeLine(frame); err == nil {
		t.Fatal("decodeLine() bad crc error = nil, want error")
	}

	cat := newCatalog(t.TempDir())
	cat.applyEntry(walEntry{Type: "unknown"})
	if len(cat.series) != 0 {
		t.Fatalf("series count = %d, want 0", len(cat.series))
	}
}

func TestAppendEntryLockedReturnsWriteError(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cat.mu.Lock()
	if err := cat.wal.Close(); err != nil {
		cat.mu.Unlock()
		t.Fatalf("Close(wal) error = %v", err)
	}
	err = cat.appendEntryLocked(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu"},
	})
	cat.wal = nil
	cat.mu.Unlock()
	if err == nil {
		t.Fatal("appendEntryLocked() error = nil, want write error")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCatalogBinaryDecodeValidationErrors(t *testing.T) {
	if _, err := encodeWALEntry(walEntry{Type: "series"}); err == nil {
		t.Fatal("encodeWALEntry(missing series) error = nil, want error")
	}
	if _, err := encodeWALEntry(walEntry{Type: "unknown"}); err == nil {
		t.Fatal("encodeWALEntry(unknown) error = nil, want error")
	}
	if _, err := decodeWALFrame(codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, []byte{99})); err == nil {
		t.Fatal("decodeWALFrame(unknown record) error = nil, want error")
	}
	if _, err := decodeSnapshot(codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, []byte{1})); err == nil {
		t.Fatal("decodeSnapshot(truncated) error = nil, want error")
	}
	if !validFieldType(model.FieldBool) || validFieldType(model.FieldType(99)) {
		t.Fatal("validFieldType() returned unexpected result")
	}
	if _, err := uint32Value("field", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	reader := newPayloadReader([]byte{1})
	if err := reader.done("catalog test"); err == nil {
		t.Fatal("payloadReader.done(trailing) error = nil, want error")
	}
}

func TestCatalogRejectsLegacySnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile(snapshot.json) error = %v", err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("Open(legacy snapshot) error = nil, want error")
	}
}

func TestCatalogBinaryDecodersRejectTruncatedPrefixes(t *testing.T) {
	snapshotPayload := encodeSnapshot(snapshot{
		NextSeriesID: 2,
		NextFieldID:  2,
		Series:       []Series{{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}}},
		Fields:       []Field{{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64}},
	})
	for size := 0; size < len(snapshotPayload); size++ {
		if _, err := decodeSnapshot(snapshotPayload[:size]); err == nil {
			t.Fatalf("decodeSnapshot(prefix %d) error = nil, want error", size)
		}
	}

	entryPayload, err := encodeWALEntry(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry() error = %v", err)
	}
	frame := codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, entryPayload)
	for size := 0; size < len(frame); size++ {
		if _, err := decodeWALFrame(frame[:size]); err == nil {
			t.Fatalf("decodeWALFrame(prefix %d) error = nil, want error", size)
		}
	}
}

func TestCatalogPayloadDecodersRejectTruncatedInnerPayload(t *testing.T) {
	snapshotFrame := encodeSnapshot(snapshot{
		NextSeriesID: 2,
		NextFieldID:  2,
		Series:       []Series{{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}}},
		Fields:       []Field{{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64}},
	})
	env, err := codec.UnmarshalEnvelope(snapshotFrame, catalogMagic, catalogVersion)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope(snapshot) error = %v", err)
	}
	for size := 0; size < len(env.Payload); size++ {
		frame := codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, env.Payload[:size])
		if _, err := decodeSnapshot(frame); err == nil {
			t.Fatalf("decodeSnapshot(inner prefix %d) error = nil, want error", size)
		}
	}

	entryPayload, err := encodeWALEntry(walEntry{
		Type:  "field",
		Field: &Field{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry(field) error = %v", err)
	}
	for size := 0; size < len(entryPayload); size++ {
		frame := codec.MarshalEnvelope(nil, catalogMagic, catalogVersion, 0, entryPayload[:size])
		if _, err := decodeWALFrame(frame); err == nil {
			t.Fatalf("decodeWALFrame(inner prefix %d) error = nil, want error", size)
		}
	}
}
