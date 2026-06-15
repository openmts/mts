package catalog

import (
	"encoding/json"
	"testing"
)

func TestDecodeLineRejectsBadCRCAndApplyEntryIgnoresUnknown(t *testing.T) {
	payload := []byte(`{"type":"series"}`)
	line, err := json.Marshal(walLine{CRC: 123, Payload: payload})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := decodeLine(line); err == nil {
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
	err = cat.appendEntryLocked(walEntry{Type: "unknown"})
	cat.wal = nil
	cat.mu.Unlock()
	if err == nil {
		t.Fatal("appendEntryLocked() error = nil, want write error")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
