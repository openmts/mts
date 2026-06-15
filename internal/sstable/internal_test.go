package sstable

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestBlockReadValidationErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocks.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := readBlock(path, blockRef{Offset: 0, Size: 3}); err == nil {
		t.Fatal("readBlock() small frame error = nil, want error")
	}

	frame := make([]byte, 8)
	binary.BigEndian.PutUint32(frame[:4], 99)
	if err := os.WriteFile(path, frame, 0600); err != nil {
		t.Fatalf("WriteFile() mismatch error = %v", err)
	}
	if _, err := readBlock(path, blockRef{Offset: 0, Size: int64(len(frame))}); err == nil {
		t.Fatal("readBlock() length mismatch error = nil, want error")
	}
	if _, err := readBlock(filepath.Join(dir, "missing.bin"), blockRef{}); err == nil {
		t.Fatal("readBlock(missing) error = nil, want error")
	}
	file := mustCreateTestFile(t, filepath.Join(dir, "closed.bin"))
	if err := file.Close(); err != nil {
		t.Fatalf("Close(closed.bin) error = %v", err)
	}
	if _, err := writeBlock(file, []byte("x")); err == nil {
		t.Fatal("writeBlock(closed) error = nil, want error")
	}
}

func TestPartUnknownEncodingsAndSortHelpers(t *testing.T) {
	dir := t.TempDir()
	timePath := filepath.Join(dir, timestampsFile)
	valuePath := filepath.Join(dir, valuesFile)
	timeFile := mustCreateTestFile(t, timePath)
	timeRef, err := writeBlock(timeFile, []byte(`{"encoding":"unknown","timestamps":[1]}`))
	if err != nil {
		t.Fatalf("writeBlock(time) error = %v", err)
	}
	if err := timeFile.Close(); err != nil {
		t.Fatalf("Close(time) error = %v", err)
	}
	valueFile := mustCreateTestFile(t, valuePath)
	valueRef, err := writeBlock(valueFile, []byte(`{"encoding":"unknown","field_id":2,"samples":[]}`))
	if err != nil {
		t.Fatalf("writeBlock(value) error = %v", err)
	}
	if err := valueFile.Close(); err != nil {
		t.Fatalf("Close(value) error = %v", err)
	}

	part := &Part{path: dir}
	if _, err := part.readTimeBlock(timeRef); err == nil {
		t.Fatal("readTimeBlock() unknown encoding error = nil, want error")
	}
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, Query{Start: 0, End: 10})
	if err == nil {
		t.Fatal("readValueColumn() unknown encoding error = nil, want error")
	}

	columns := []model.ColumnData{
		{SeriesID: 2, FieldID: 1},
		{SeriesID: 1, FieldID: 3},
		{SeriesID: 1, FieldID: 2},
	}
	sortColumns(columns)
	if columns[0].SeriesID != 1 || columns[0].FieldID != 2 {
		t.Fatalf("first sorted column = (%d,%d), want (1,2)", columns[0].SeriesID, columns[0].FieldID)
	}
	if !containsSeries(nil, 1) || !containsField(nil, 1) {
		t.Fatal("nil filters should match")
	}
	if rowMatches(indexRow{SeriesID: 1, MinTime: 100, MaxTime: 200}, Query{Start: 0, End: 10}) {
		t.Fatal("rowMatches() matched non-overlapping time range")
	}
	if !rowMatches(indexRow{SeriesID: 1, MinTime: 1, MaxTime: 2}, Query{Start: 0, End: 10}) {
		t.Fatal("rowMatches() did not match overlapping range")
	}
}

func TestPartDecodeErrors(t *testing.T) {
	dir := t.TempDir()
	timeFile := mustCreateTestFile(t, filepath.Join(dir, timestampsFile))
	timeRef, err := writeBlock(timeFile, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(time) error = %v", err)
	}
	if err := timeFile.Close(); err != nil {
		t.Fatalf("Close(time) error = %v", err)
	}
	valueFile := mustCreateTestFile(t, filepath.Join(dir, valuesFile))
	valueRef, err := writeBlock(valueFile, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(value) error = %v", err)
	}
	if err := valueFile.Close(); err != nil {
		t.Fatalf("Close(value) error = %v", err)
	}
	part := &Part{path: dir}
	if _, err := part.readTimeBlock(timeRef); err == nil {
		t.Fatal("readTimeBlock(bad json) error = nil, want error")
	}
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, Query{Start: 0, End: 10})
	if err == nil {
		t.Fatal("readValueColumn(bad json) error = nil, want error")
	}
	missingPart := &Part{path: filepath.Join(dir, "missing")}
	if _, err := missingPart.readTimeBlock(blockRef{}); err == nil {
		t.Fatal("readTimeBlock(missing file) error = nil, want error")
	}
	filtered := filterSamples(1, valueBlock{
		FieldID:   2,
		FieldType: model.FieldInt64,
		Samples: []model.VersionedSample{
			{Timestamp: 100, WriteSeq: 1, Value: model.Int64Value(1)},
		},
	}, Query{Start: 0, End: 10})
	if len(filtered.Samples) != 0 {
		t.Fatalf("filtered sample count = %d, want 0", len(filtered.Samples))
	}
}

func TestOpenPartRejectsBadIndexJSON(t *testing.T) {
	dir := t.TempDir()
	indexFileHandle := mustCreateTestFile(t, filepath.Join(dir, indexFile))
	indexRef, err := writeBlock(indexFileHandle, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(index) error = %v", err)
	}
	if err := indexFileHandle.Close(); err != nil {
		t.Fatalf("Close(index) error = %v", err)
	}
	meta := metadata{
		FormatVersion: 1,
		Part:          PartMeta{ID: "bad-index"},
		IndexRef:      indexRef,
	}
	if err := writeMetadata(dir, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	if _, err := OpenPart(dir); err == nil {
		t.Fatal("OpenPart(bad index json) error = nil, want error")
	}
}

func TestWriteJSONBlockMarshalError(t *testing.T) {
	if _, err := writeJSONBlock(filepath.Join(t.TempDir(), "index.bin"), make(chan int)); err == nil {
		t.Fatal("writeJSONBlock() error = nil, want marshal error")
	}
	if err := writeMetadata("bad\x00path", metadata{}); err == nil {
		t.Fatal("writeMetadata(invalid) error = nil, want error")
	}
	if err := ensureStringsFile("bad\x00path"); err == nil {
		t.Fatal("ensureStringsFile(invalid) error = nil, want error")
	}
	if _, err := openWritable("bad\x00path"); err == nil {
		t.Fatal("openWritable(invalid) error = nil, want error")
	}
	meta := newMetadata(0, "bad")
	if err := writePartIndexes("bad\x00path", &meta, nil); err == nil {
		t.Fatal("writePartIndexes(invalid) error = nil, want error")
	}
}

func TestWritePartPropagatesValueMarshalError(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(math.NaN())},
			},
		},
	}
	if _, err := WritePart(t.TempDir(), 0, "sst-nan", columns); err == nil {
		t.Fatal("WritePart(NaN) error = nil, want marshal error")
	}
}

func TestOpenPartFilesAndCloseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, valuesFile), 0700); err != nil {
		t.Fatalf("Mkdir(values.bin) error = %v", err)
	}
	if _, err := openPartFiles(dir); err == nil {
		t.Fatal("openPartFiles() error = nil, want values open error")
	}
	timestamps := mustCreateTestFile(t, filepath.Join(t.TempDir(), "timestamps.bin"))
	values := mustCreateTestFile(t, filepath.Join(t.TempDir(), "values.bin"))
	if err := timestamps.Close(); err != nil {
		t.Fatalf("Close(timestamps) error = %v", err)
	}
	if err := values.Close(); err != nil {
		t.Fatalf("Close(values) error = %v", err)
	}
	files := &partFiles{timestamps: timestamps, values: values}
	if err := files.close(); err == nil {
		t.Fatal("partFiles.close() error = nil, want error")
	}
}

func mustCreateTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	return file
}
