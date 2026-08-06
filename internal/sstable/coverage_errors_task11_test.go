package sstable

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func TestLogicalComponentFallbacksAndCorruptPackErrors(t *testing.T) {
	dir := t.TempDir()
	writeCoverageFile(t, filepath.Join(dir, indexFile), []byte("index"))
	if err := ensurePartComponentPresent(dir, indexFile, nil); err != nil {
		t.Fatalf("ensurePartComponentPresent(file) error = %v", err)
	}
	if size, err := partComponentSize(dir, indexFile, nil); err != nil || size != 5 {
		t.Fatalf("partComponentSize(file) = %d, %v; want 5, nil", size, err)
	}
	if err := os.Remove(filepath.Join(dir, indexFile)); err != nil {
		t.Fatalf("Remove(index fixture) error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, indexFile), 0700); err != nil {
		t.Fatalf("Mkdir(index fixture) error = %v", err)
	}
	if err := ensurePartComponentPresent(dir, indexFile, nil); err == nil {
		t.Fatal("ensurePartComponentPresent(directory) error = nil")
	}
	if _, err := partComponentSize(dir, indexFile, nil); err == nil {
		t.Fatal("partComponentSize(directory) error = nil")
	}
	if err := ensurePartComponentPresent(dir, "missing.bin", nil); err == nil {
		t.Fatal("ensurePartComponentPresent(missing) error = nil")
	}
	if _, err := partComponentSize(dir, "missing.bin", nil); err == nil {
		t.Fatal("partComponentSize(missing) error = nil")
	}

	corruptDir := t.TempDir()
	writeCoverageFile(t, filepath.Join(corruptDir, packFile), []byte("bad-pack"))
	if _, err := PartLogicalComponentSize(corruptDir, indexFile); err == nil {
		t.Fatal("PartLogicalComponentSize(corrupt pack) error = nil")
	}
	if err := OverwriteLogicalComponentAt(corruptDir, indexFile, 0, []byte{1}); err == nil {
		t.Fatal("OverwriteLogicalComponentAt(corrupt pack) error = nil")
	}
	if err := RemoveLogicalComponent(corruptDir, indexFile); err == nil {
		t.Fatal("RemoveLogicalComponent(corrupt pack) error = nil")
	}

	metadataDir := t.TempDir()
	writeCoverageFile(t, filepath.Join(metadataDir, metadataFile), []byte("meta"))
	if err := RemoveLogicalComponent(metadataDir, metadataFile); err != nil {
		t.Fatalf("RemoveLogicalComponent(metadata) error = %v", err)
	}
	if _, err := PartLogicalComponentSize(metadataDir, metadataFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("PartLogicalComponentSize(removed metadata) error = %v", err)
	}
}

func TestSeriesBatchReaderPropagatesCorruptComponents(t *testing.T) {
	dir := t.TempDir()
	valueMeta, err := WritePart(dir, 0, "sst-reader-value-error", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(1.5)),
	})
	if err != nil {
		t.Fatalf("WritePart(values) error = %v", err)
	}
	valuePart, err := OpenPart(valueMeta.Path)
	if err != nil {
		t.Fatalf("OpenPart(values) error = %v", err)
	}
	valueReader, err := NewSeriesBatchReader(valuePart, Query{Start: 0, End: 100})
	if err != nil {
		closeErr := valuePart.Close()
		t.Fatalf("NewSeriesBatchReader(values) error = %v close = %v", err, closeErr)
	}
	valueSize, err := PartLogicalComponentSize(valueMeta.Path, valuesFile)
	if err != nil {
		closeErr := valuePart.Close()
		t.Fatalf("PartLogicalComponentSize(values) error = %v close = %v", err, closeErr)
	}
	if err := OverwriteLogicalComponentAt(valueMeta.Path, valuesFile, 0, filledBytes(valueSize, 0xff)); err != nil {
		closeErr := valuePart.Close()
		t.Fatalf("OverwriteLogicalComponentAt(values) error = %v close = %v", err, closeErr)
	}
	if _, err := valueReader.QuerySeriesIDs([]uint64{1}); err == nil {
		closeErr := valuePart.Close()
		t.Fatalf("QuerySeriesIDs(corrupt values) error = nil close = %v", closeErr)
	}
	if _, err := valueReader.QuerySeriesID(1); err == nil {
		closeErr := valuePart.Close()
		t.Fatalf("QuerySeriesID(corrupt values) error = nil close = %v", closeErr)
	}
	if err := valuePart.Close(); err != nil {
		t.Fatalf("Close(values part) error = %v", err)
	}

	indexMeta, err := WritePart(dir, 0, "sst-reader-index-error", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(2.5)),
	})
	if err != nil {
		t.Fatalf("WritePart(index) error = %v", err)
	}
	indexPart, err := OpenPart(indexMeta.Path)
	if err != nil {
		t.Fatalf("OpenPart(index) error = %v", err)
	}
	indexSize, err := PartLogicalComponentSize(indexMeta.Path, indexFile)
	if err != nil {
		closeErr := indexPart.Close()
		t.Fatalf("PartLogicalComponentSize(index) error = %v close = %v", err, closeErr)
	}
	if err := OverwriteLogicalComponentAt(indexMeta.Path, indexFile, 0, filledBytes(indexSize, 0xff)); err != nil {
		closeErr := indexPart.Close()
		t.Fatalf("OverwriteLogicalComponentAt(index) error = %v close = %v", err, closeErr)
	}
	if _, err := NewSeriesBatchReader(indexPart, Query{Start: 0, End: 100}); err == nil {
		closeErr := indexPart.Close()
		t.Fatalf("NewSeriesBatchReader(corrupt index) error = nil close = %v", closeErr)
	}
	if err := indexPart.Close(); err != nil {
		t.Fatalf("Close(index part) error = %v", err)
	}
	filteredReader := &SeriesBatchReader{
		query: Query{Start: 30, End: 40},
		rows:  []indexRow{{SeriesID: 1, MinTime: 10, MaxTime: 20}},
	}
	if columns, err := filteredReader.QuerySeriesID(1); err != nil || len(columns) != 0 {
		t.Fatalf("QuerySeriesID(filtered row) = %#v, %v; want empty, nil", columns, err)
	}
}

func TestLoadPartComponentSizesUsesEmbeddedValues(t *testing.T) {
	dir := t.TempDir()
	writeCoverageFile(t, filepath.Join(dir, metadataFile), []byte("meta"))
	writeCoverageFile(t, filepath.Join(dir, indexFile), []byte("index"))
	meta := metadata{
		Components:     []string{metadataFile, indexFile},
		ComponentSizes: map[string]int64{indexFile: 5},
	}
	sizes, err := loadPartComponentSizes(dir, meta, nil, true)
	if err != nil || sizes[metadataFile] != 0 || sizes[indexFile] != 5 {
		t.Fatalf("loadPartComponentSizes(embedded) = %v, %v", sizes, err)
	}

	missing := metadata{
		Components:     []string{metadataFile, "missing.bin"},
		ComponentSizes: map[string]int64{metadataFile: 4},
	}
	if _, err := loadPartComponentSizes(dir, missing, nil, true); err == nil {
		t.Fatal("loadPartComponentSizes(missing) error = nil")
	}
}

func TestPartPackParserAdditionalErrors(t *testing.T) {
	if _, _, err := decodePartPackHeader([]byte("short")); err == nil {
		t.Fatal("decodePartPackHeader(short) error = nil")
	}
	missingNameLength := append([]byte(packMagic), 1)
	if _, _, err := decodePartPackHeader(missingNameLength); err == nil {
		t.Fatal("decodePartPackHeader(missing name length) error = nil")
	}

	file, err := os.CreateTemp(t.TempDir(), "closed-pack-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := readPartPackSections(file); err == nil {
		t.Fatal("readPartPackSections(closed) error = nil")
	}
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open(directory) error = %v", err)
	}
	if _, err := readPartPackSections(directory); err == nil {
		closeErr := directory.Close()
		t.Fatalf("readPartPackSections(directory) error = nil close = %v", closeErr)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("Close(directory) error = %v", err)
	}
}

func TestWritePartPackSyncAndPartWriterValidation(t *testing.T) {
	syncDir := t.TempDir()
	writePackComponents(t, syncDir)
	sizes, err := writePartPack(syncDir, true)
	if err != nil || len(sizes) != len(packSectionOrder)+1 {
		t.Fatalf("writePartPack(sync) = %v, %v", sizes, err)
	}
	if _, err := os.Stat(filepath.Join(syncDir, packFile)); err != nil {
		t.Fatalf("Stat(pack) error = %v", err)
	}

	faultDir := t.TempDir()
	writePackComponents(t, faultDir)
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpSync, nil)
	fs.FailNext(faultinject.OpSync, nil)
	fs.FailNext(faultinject.OpSync, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	_, err = writePartPack(faultDir, true)
	restore()
	if err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("writePartPack(sync fault) error = %v, want permission error", err)
	}

	writer, err := NewPartWriter(t.TempDir(), 0, "empty", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(empty) error = %v", err)
	}
	if err := writer.AddSeries(nil); err != nil {
		t.Fatalf("AddSeries(nil) error = %v", err)
	}
	if _, err := writer.Close(); err == nil {
		t.Fatal("Close(empty writer) error = nil")
	}
	if _, err := writer.Close(); err == nil {
		t.Fatal("Close(already closed writer) error = nil")
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("Abort(closed writer) error = %v", err)
	}

	invalidWriter, err := NewPartWriter(t.TempDir(), 0, "invalid", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(invalid) error = %v", err)
	}
	mixed := []model.ColumnData{
		columnWithField(1, 1, model.Int64Value(1)),
		columnWithField(2, 1, model.Int64Value(2)),
	}
	if err := invalidWriter.AddSeries(mixed); err == nil {
		t.Fatal("AddSeries(mixed series) error = nil")
	}
	if err := invalidWriter.AddSeries([]model.ColumnData{{SeriesID: 1, FieldID: 1}}); err == nil {
		t.Fatal("AddSeries(empty column) error = nil")
	}
	if err := invalidWriter.Abort(); err != nil {
		t.Fatalf("Abort(invalid writer) error = %v", err)
	}

	var nilWriter *PartWriter
	if err := nilWriter.AddSeries(nil); err == nil {
		t.Fatal("nil AddSeries() error = nil")
	}
	if _, err := nilWriter.Close(); err == nil {
		t.Fatal("nil Close() error = nil")
	}
	if err := nilWriter.Abort(); err != nil {
		t.Fatalf("nil Abort() error = %v", err)
	}
}

func filledBytes(size int64, value byte) []byte {
	data := make([]byte, int(size))
	for index := range data {
		data[index] = value
	}
	return data
}

func writePackComponents(t *testing.T, dir string) {
	t.Helper()
	for _, name := range packSectionOrder {
		writeCoverageFile(t, filepath.Join(dir, name), []byte(name))
	}
}
