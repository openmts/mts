package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openmts/mts/internal/engine"
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

func TestStorageToolSnapshotRestoreCommands(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	eng, err := engine.Open(ctx, model.Options{
		Path:               source,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(0.7)},
	}}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close(source) error = %v", err)
	}

	snapshotDir := filepath.Join(t.TempDir(), "snapshot")
	var out bytes.Buffer
	var errOut bytes.Buffer
	if code := run([]string{"snapshot", source, snapshotDir}, &out, &errOut); code != 0 {
		t.Fatalf("snapshot exit = %d output = %s error = %s", code, out.String(), errOut.String())
	}
	var snapshotResult struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(out.Bytes(), &snapshotResult); err != nil {
		t.Fatalf("Unmarshal(snapshot) error = %v output=%s", err, out.String())
	}
	if snapshotResult.Source == "" || snapshotResult.Target == "" {
		t.Fatalf("snapshot result = %#v, want source and target", snapshotResult)
	}

	restoreDir := filepath.Join(t.TempDir(), "restore")
	out.Reset()
	errOut.Reset()
	if code := run([]string{"restore", snapshotDir, restoreDir}, &out, &errOut); code != 0 {
		t.Fatalf("restore exit = %d output = %s error = %s", code, out.String(), errOut.String())
	}
	restored, err := engine.Open(ctx, model.Options{Path: restoreDir})
	if err != nil {
		t.Fatalf("Open(restored) error = %v", err)
	}
	rows, err := restored.QueryRows(ctx, model.Query{Measurement: "cpu"})
	if err != nil {
		closeErr := restored.Close(ctx)
		t.Fatalf("QueryRows(restored) error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Fields["usage"].Float64 != 0.7 {
		closeErr := restored.Close(ctx)
		t.Fatalf("restored rows = %#v, want one usage row close=%v", rows, closeErr)
	}
	if err := restored.Close(ctx); err != nil {
		t.Fatalf("Close(restored) error = %v", err)
	}
}

func TestStorageToolMigrateAndErrorPaths(t *testing.T) {
	dir := t.TempDir()
	if err := sstable.WriteManifest(dir, sstable.Manifest{Sequence: 1}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	if code := run([]string{"migrate", "--dry-run", dir}, &out, &errOut); code != 0 {
		t.Fatalf("migrate dry-run exit = %d output = %s error = %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"applied":false`) {
		t.Fatalf("migrate dry-run output = %s, want applied false", out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"migrate", "--apply", dir}, &out, &errOut); code != 0 {
		t.Fatalf("migrate apply exit = %d output = %s error = %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"applied":true`) {
		t.Fatalf("migrate apply output = %s, want applied true", out.String())
	}

	for _, args := range [][]string{
		{},
		{"unknown"},
		{"check"},
		{"repair", "--dry-run", "--apply", dir},
		{"migrate", "--dry-run", "--apply", dir},
		{"snapshot", dir},
		{"restore", dir},
	} {
		out.Reset()
		errOut.Reset()
		if code := run(args, &out, &errOut); code != 2 {
			t.Fatalf("run(%v) exit = %d, want 2 output=%s error=%s", args, code, out.String(), errOut.String())
		}
	}
}

func TestRunMainSetsSlogDefaultAndReturnsUsage(t *testing.T) {
	previous := slog.Default()
	defer slog.SetDefault(previous)
	marker := slog.New(slog.NewTextHandler(io.Discard, nil))
	slog.SetDefault(marker)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if code := runMain(nil, &out, &errOut); code != 2 {
		t.Fatalf("runMain(nil) exit = %d, want 2", code)
	}
	if slog.Default() == marker {
		t.Fatal("runMain() should replace the default slog logger")
	}
	if !strings.Contains(errOut.String(), "usage: mts-storage") {
		t.Fatalf("stderr = %q, want usage", errOut.String())
	}
}

func TestWriteJSONReportsErrors(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if code := writeJSON(&out, &errOut, nil, errors.New("boom")); code != 1 {
		t.Fatalf("writeJSON(error) code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "boom") {
		t.Fatalf("stderr = %q, want boom", errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := writeJSON(&out, &errOut, map[string]any{"bad": func() {}}, nil); code != 1 {
		t.Fatalf("writeJSON(encode error) code = %d, want 1", code)
	}
}
