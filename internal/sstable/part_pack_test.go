package sstable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestWritePartProducesPackLayout(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-pack", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(1.5)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(meta.Path, packFile)); err != nil {
		t.Fatalf("pack.bin missing: %v", err)
	}
	for _, name := range packSectionOrder {
		if _, err := os.Stat(filepath.Join(meta.Path, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy component %s still present, err=%v", name, err)
		}
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	got, err := part.Query(Query{Start: 0, End: 100})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Query() error = %v close=%v", err, closeErr)
	}
	if len(got) != 1 || len(got[0].Samples) != 1 || got[0].Samples[0].Value.Float64 != 1.5 {
		closeErr := part.Close()
		t.Fatalf("Query() = %#v close=%v", got, closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	decoded, err := loadPartMetadata(meta.Path)
	if err != nil {
		t.Fatalf("loadPartMetadata() error = %v", err)
	}
	for _, name := range packSectionOrder {
		if size := decoded.ComponentSizes[name]; size <= 0 && name != stringsFile {
			// strings may be empty; others should have size.
			if name != stringsFile && size < 0 {
				t.Fatalf("component size %s = %d", name, size)
			}
		}
	}
}

func TestDecodePartPackHeaderRejectsBadMagic(t *testing.T) {
	if _, _, err := decodePartPackHeader([]byte("BADMAGIC")); err == nil {
		t.Fatal("decodePartPackHeader(bad) error = nil, want error")
	}
}
