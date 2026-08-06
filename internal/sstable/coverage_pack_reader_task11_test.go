package sstable

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestPackReaderAndLogicalComponentContracts(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-pack-contract", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(1.5)),
		columnWithField(2, 2, model.Float64Value(2.5)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	reader, err := part.NewSeriesBatchReader(Query{Start: 0, End: 100})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Part.NewSeriesBatchReader() error = %v close = %v", err, closeErr)
	}
	if reader.SeriesCount() != 2 {
		closeErr := part.Close()
		t.Fatalf("SeriesCount() = %d, want 2 close = %v", reader.SeriesCount(), closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Part.Close() error = %v", err)
	}

	file, sections, err := openPartPack(meta.Path)
	if err != nil {
		t.Fatalf("openPartPack() error = %v", err)
	}
	section, err := packSectionFile(file, sections, indexFile)
	if err != nil {
		closeErr := file.Close()
		t.Fatalf("packSectionFile(index) error = %v close = %v", err, closeErr)
	}
	if section.Size() != sections[indexFile].Size {
		closeErr := file.Close()
		t.Fatalf("Size() = %d, want %d close = %v", section.Size(), sections[indexFile].Size, closeErr)
	}
	buf := make([]byte, section.Size()+1)
	n, err := section.ReadAt(buf, 0)
	if err != nil || int64(n) != section.Size() {
		closeErr := file.Close()
		t.Fatalf("ReadAt(full) = %d, %v; want %d, nil close = %v", n, err, section.Size(), closeErr)
	}
	if _, err := section.ReadAt(make([]byte, 1), -1); err == nil {
		closeErr := file.Close()
		t.Fatalf("ReadAt(negative) error = nil close = %v", closeErr)
	}
	if _, err := section.ReadAt(make([]byte, 1), section.Size()); !errors.Is(err, io.EOF) {
		closeErr := file.Close()
		t.Fatalf("ReadAt(end) error = %v, want EOF close = %v", err, closeErr)
	}
	var nilSection *sectionReader
	if _, err := nilSection.ReadAt(make([]byte, 1), 0); err == nil || nilSection.Size() != 0 {
		closeErr := file.Close()
		t.Fatalf("nil section result error = %v size = %d close = %v", err, nilSection.Size(), closeErr)
	}
	if _, err := packSectionFile(file, sections, "missing.bin"); err == nil {
		closeErr := file.Close()
		t.Fatalf("packSectionFile(missing) error = nil close = %v", closeErr)
	}

	metadataSize, err := PartLogicalComponentSize(meta.Path, metadataFile)
	if err != nil || metadataSize <= 0 {
		closeErr := file.Close()
		t.Fatalf("PartLogicalComponentSize(metadata) = %d, %v close = %v", metadataSize, err, closeErr)
	}
	indexSize, err := PartLogicalComponentSize(meta.Path, indexFile)
	if err != nil || indexSize != section.Size() {
		closeErr := file.Close()
		t.Fatalf("PartLogicalComponentSize(index) = %d, %v; want %d close = %v", indexSize, err, section.Size(), closeErr)
	}
	if _, err := PartLogicalComponentSize(meta.Path, "missing.bin"); err == nil {
		closeErr := file.Close()
		t.Fatalf("PartLogicalComponentSize(missing) error = nil close = %v", closeErr)
	}
	sizes, ok := collectPackLogicalSizes(meta.Path)
	if !ok || sizes[indexFile] != indexSize {
		closeErr := file.Close()
		t.Fatalf("collectPackLogicalSizes() = %v, %v close = %v", sizes, ok, closeErr)
	}
	if err := OverwriteLogicalComponentAt(meta.Path, "missing.bin", 0, []byte{1}); err == nil {
		closeErr := file.Close()
		t.Fatalf("OverwriteLogicalComponentAt(missing) error = nil close = %v", closeErr)
	}
	if err := RemoveLogicalComponent(meta.Path, "missing.bin"); err == nil {
		closeErr := file.Close()
		t.Fatalf("RemoveLogicalComponent(missing) error = nil close = %v", closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("pack file Close() error = %v", err)
	}
}

func TestLegacyLogicalComponentAndSizeFallbacks(t *testing.T) {
	dir := t.TempDir()
	writeCoverageFile(t, filepath.Join(dir, metadataFile), []byte("meta"))
	writeCoverageFile(t, filepath.Join(dir, valuesFile), []byte("value"))

	size, err := PartLogicalComponentSize(dir, valuesFile)
	if err != nil || size != 5 {
		t.Fatalf("PartLogicalComponentSize(legacy) = %d, %v; want 5, nil", size, err)
	}
	if err := OverwriteLogicalComponentAt(dir, valuesFile, 1, []byte("X")); err != nil {
		t.Fatalf("OverwriteLogicalComponentAt(legacy) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, valuesFile))
	if err != nil || string(data) != "vXlue" {
		t.Fatalf("ReadFile(values) = %q, %v; want vXlue", data, err)
	}
	if err := RemoveLogicalComponent(dir, valuesFile); err != nil {
		t.Fatalf("RemoveLogicalComponent(legacy) error = %v", err)
	}
	if _, err := PartLogicalComponentSize(dir, valuesFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("PartLogicalComponentSize(removed) error = %v, want not exist", err)
	}

	files := &partReadFiles{
		sections:   map[string]packSection{metaindexFile: {Size: 7}},
		index:      &sectionReader{size: 8},
		timestamps: &sectionReader{size: 9},
		values:     &sectionReader{size: 10},
	}
	checks := map[string]int64{metaindexFile: 7, indexFile: 8, timestampsFile: 9, valuesFile: 10}
	for name, want := range checks {
		got, ok := logicalComponentSize(files, name)
		if !ok || got != want {
			t.Fatalf("logicalComponentSize(%s) = %d, %v; want %d, true", name, got, ok, want)
		}
	}
	if _, ok := logicalComponentSize(nil, indexFile); ok {
		t.Fatal("logicalComponentSize(nil) ok = true")
	}
	if _, ok := logicalComponentSize(files, "missing.bin"); ok {
		t.Fatal("logicalComponentSize(missing) ok = true")
	}
	if err := ensurePartComponentPresent(dir, metaindexFile, files); err != nil {
		t.Fatalf("ensurePartComponentPresent(section) error = %v", err)
	}
	if got, err := partComponentSize(dir, metaindexFile, files); err != nil || got != 7 {
		t.Fatalf("partComponentSize(section) = %d, %v", got, err)
	}
	negative := &partReadFiles{sections: map[string]packSection{indexFile: {Size: -1}}}
	if err := ensurePartComponentPresent(dir, indexFile, negative); err == nil {
		t.Fatal("ensurePartComponentPresent(negative section) error = nil")
	}

	componentDir := t.TempDir()
	writeCoverageFile(t, filepath.Join(componentDir, metadataFile), []byte("meta"))
	writeCoverageFile(t, filepath.Join(componentDir, indexFile), []byte("index"))
	meta := metadata{
		Components:     []string{metadataFile, indexFile},
		ComponentSizes: map[string]int64{metadataFile: 4},
	}
	sizes, err := loadPartComponentSizes(componentDir, meta, nil, true)
	if err != nil || sizes[metadataFile] != 4 || sizes[indexFile] != 5 {
		t.Fatalf("loadPartComponentSizes(partial) = %v, %v", sizes, err)
	}
	meta.ComponentSizes = nil
	sizes, err = loadPartComponentSizes(componentDir, meta, nil, false)
	if err != nil || sizes[metadataFile] != 4 || sizes[indexFile] != 5 {
		t.Fatalf("loadPartComponentSizes(fallback) = %v, %v", sizes, err)
	}

	directoryFixture := t.TempDir()
	if err := os.Mkdir(filepath.Join(directoryFixture, metadataFile), 0700); err != nil {
		t.Fatalf("Mkdir(metadata component) error = %v", err)
	}
	if err := ensurePartComponentPresent(directoryFixture, metadataFile, nil); err == nil {
		t.Fatal("ensurePartComponentPresent(directory) error = nil")
	}
	if _, err := partComponentSize(directoryFixture, metadataFile, nil); err == nil {
		t.Fatal("partComponentSize(directory) error = nil")
	}
}

func TestCloseLegacyOpenErrorJoinsCloseFailure(t *testing.T) {
	closed, err := os.CreateTemp(t.TempDir(), "closed-*")
	if err != nil {
		t.Fatalf("CreateTemp(closed) error = %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("Close(closed fixture) error = %v", err)
	}
	open, err := os.CreateTemp(t.TempDir(), "open-*")
	if err != nil {
		t.Fatalf("CreateTemp(open) error = %v", err)
	}
	openErr := errors.New("open failed")
	got := closeLegacyOpenError(openErr, nil, closed, open)
	if !errors.Is(got, openErr) || !errors.Is(got, os.ErrClosed) {
		t.Fatalf("closeLegacyOpenError() = %v, want joined open and close errors", got)
	}
}

func TestPartPackRejectsMalformedLayouts(t *testing.T) {
	if _, err := encodePartPack([]packSection{{Name: "one", Size: 1}}, nil); err == nil {
		t.Fatal("encodePartPack(mismatched payloads) error = nil")
	}

	cases := []struct {
		name string
		data []byte
	}{
		{name: "missing count", data: []byte(packMagic)},
		{name: "truncated name", data: appendPackHeaderField(2, []byte("a"), nil)},
		{name: "missing size", data: appendPackHeaderField(1, []byte("a"), nil)},
		{name: "overflow size", data: appendPackHeaderField(1, []byte("a"), binary.AppendUvarint(nil, ^uint64(0)))},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if _, _, err := decodePartPackHeader(item.data); err == nil {
				t.Fatal("decodePartPackHeader() error = nil")
			}
		})
	}

	dir := t.TempDir()
	if _, _, err := openPartPack(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("openPartPack(missing) error = nil")
	}
	writeCoverageFile(t, filepath.Join(dir, packFile), []byte("short"))
	if _, _, err := openPartPack(dir); err == nil {
		t.Fatal("openPartPack(short) error = nil")
	}

	incomplete, err := encodePartPack(
		[]packSection{{Name: indexFile, Size: 1}},
		[][]byte{{1}},
	)
	if err != nil {
		t.Fatalf("encodePartPack(incomplete) error = %v", err)
	}
	writeCoverageFile(t, filepath.Join(dir, packFile), incomplete)
	if sizes, ok := collectPackLogicalSizes(dir); ok || sizes != nil {
		t.Fatalf("collectPackLogicalSizes(incomplete) = %v, %v; want nil, false", sizes, ok)
	}

	overflow, err := encodePartPack(
		[]packSection{{Name: indexFile, Size: 2}},
		[][]byte{{1}},
	)
	if err != nil {
		t.Fatalf("encodePartPack(overflow) error = %v", err)
	}
	writeCoverageFile(t, filepath.Join(dir, packFile), overflow)
	if _, _, err := openPartPack(dir); err == nil {
		t.Fatal("openPartPack(section exceeds file) error = nil")
	}

	extra, err := encodePartPack(
		[]packSection{{Name: indexFile, Size: 1}},
		[][]byte{{1, 2}},
	)
	if err != nil {
		t.Fatalf("encodePartPack(extra payload) error = %v", err)
	}
	writeCoverageFile(t, filepath.Join(dir, packFile), extra)
	if _, _, err := openPartPack(dir); err == nil {
		t.Fatal("openPartPack(payload mismatch) error = nil")
	}
}

func appendPackHeaderField(nameLen uint64, name []byte, size []byte) []byte {
	data := append([]byte(packMagic), 1)
	data = binary.AppendUvarint(data, nameLen)
	data = append(data, name...)
	return append(data, size...)
}

func writeCoverageFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
