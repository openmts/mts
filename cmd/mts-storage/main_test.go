package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/sstable"
)

func TestStorageToolCheckRepairCommands(t *testing.T) {
	dir := t.TempDir()
	kept, err := sstable.WritePart(dir, 0, "sst-kept", []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{
			Timestamp: 1,
			WriteSeq:  1,
			Value:     model.Float64Value(1),
		}},
	}})
	if err != nil {
		t.Fatalf("WritePart(kept) error = %v", err)
	}
	if _, err := sstable.WritePart(dir, 0, "sst-orphan", []model.ColumnData{{
		SeriesID:  2,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{
			Timestamp: 1,
			WriteSeq:  1,
			Value:     model.Float64Value(2),
		}},
	}}); err != nil {
		t.Fatalf("WritePart(orphan) error = %v", err)
	}
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 1, Parts: []sstable.PartMeta{kept}}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	var out bytes.Buffer
	if code := run([]string{"check", dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("check exit = %d output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), "orphan part") {
		t.Fatalf("check output = %s, want orphan part", out.String())
	}
	out.Reset()
	if code := run([]string{"repair", "--dry-run", dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("repair dry-run exit = %d output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"applied":false`) {
		t.Fatalf("repair dry-run output = %s, want applied false", out.String())
	}
	out.Reset()
	if code := run([]string{"repair", "--apply", dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("repair apply exit = %d output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"applied":true`) {
		t.Fatalf("repair apply output = %s, want applied true", out.String())
	}
}
