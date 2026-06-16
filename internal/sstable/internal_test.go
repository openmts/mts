package sstable

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/codec"
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

func TestBlockFramePoolLargeBufferBranch(t *testing.T) {
	frame := borrowBlockFrame(maxPooledBlockFrameBytes + 1)
	if len(frame) != maxPooledBlockFrameBytes+1 {
		t.Fatalf("borrowBlockFrame() len = %d, want %d", len(frame), maxPooledBlockFrameBytes+1)
	}
	releaseBlockFrame(frame)
	small := borrowBlockFrame(16)
	if len(small) != 16 {
		t.Fatalf("borrowBlockFrame(small) len = %d, want 16", len(small))
	}
	releaseBlockFrame(small)
}

func TestBlockFramePoolReusesSmallBuffer(t *testing.T) {
	frame := borrowBlockFrame(32)
	if len(frame) != 32 {
		t.Fatalf("borrowBlockFrame() len = %d, want 32", len(frame))
	}
	frame[0] = 7
	releaseBlockFrame(frame)

	reused := borrowBlockFrame(16)
	if len(reused) != 16 {
		t.Fatalf("borrowBlockFrame(reused) len = %d, want 16", len(reused))
	}
	if cap(reused) < 32 {
		t.Fatalf("borrowBlockFrame(reused) cap = %d, want at least 32", cap(reused))
	}
	releaseBlockFrame(reused)
}

func TestBlockPayloadReleaseDoesNotCorruptCopiedData(t *testing.T) {
	dir := t.TempDir()
	file := mustCreateTestFile(t, filepath.Join(dir, "blocks.bin"))
	ref, err := writeBlock(file, []byte("payload"))
	if err != nil {
		t.Fatalf("writeBlock() error = %v", err)
	}
	payload, err := readBlockPayloadFrom(file, ref)
	if err != nil {
		t.Fatalf("readBlockPayloadFrom() error = %v", err)
	}
	copied := append([]byte(nil), payload.Bytes()...)
	payload.Release()
	if string(copied) != "payload" {
		t.Fatalf("copied payload = %q, want payload", string(copied))
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(blocks.bin) error = %v", err)
	}
}

func TestBlockWriterWritesSequentialOffsets(t *testing.T) {
	dir := t.TempDir()
	file := mustCreateTestFile(t, filepath.Join(dir, "blocks.bin"))
	writer, err := newBlockWriter(file)
	if err != nil {
		t.Fatalf("newBlockWriter() error = %v", err)
	}
	first, err := writer.write([]byte("first"))
	if err != nil {
		t.Fatalf("writer.write(first) error = %v", err)
	}
	second, err := writer.write([]byte("second"))
	if err != nil {
		t.Fatalf("writer.write(second) error = %v", err)
	}
	if second.Offset != first.Offset+first.Size {
		t.Fatalf("second offset = %d, want %d", second.Offset, first.Offset+first.Size)
	}
	got, err := readBlockFrom(file, second)
	if err != nil {
		t.Fatalf("readBlockFrom(second) error = %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("second payload = %q, want second", string(got))
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(blocks.bin) error = %v", err)
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
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, nil, Query{Start: 0, End: 10})
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
	if partMatches(PartMeta{MinTime: 100, MaxTime: 200}, nil, Query{Start: 0, End: 10}) {
		t.Fatal("partMatches() matched non-overlapping time range")
	}
	if partMatches(PartMeta{MinTime: 0, MaxTime: 10, MinSeriesID: 10, MaxSeriesID: 20}, nil, Query{
		Start:     0,
		End:       10,
		SeriesIDs: map[uint64]struct{}{1: {}},
	}) {
		t.Fatal("partMatches() matched non-overlapping series range")
	}
	if partMatches(PartMeta{MinTime: 0, MaxTime: 10}, []metaIndexRow{{FieldIDs: []uint32{1}}}, Query{
		Start:    0,
		End:      10,
		FieldIDs: map[uint32]struct{}{2: {}},
	}) {
		t.Fatal("partMatches() matched non-overlapping field IDs")
	}
}

func TestGroupColumnsSortsUnsortedSamplesWithoutMutatingInput(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
	}
	grouped := groupColumns(columns)
	got := grouped[1][0].Samples
	if len(got) != 2 || got[0].Timestamp != 10 || got[1].Timestamp != 20 {
		t.Fatalf("grouped samples = %#v, want sorted timestamps 10,20", got)
	}
	if columns[0].Samples[0].Timestamp != 20 {
		t.Fatalf("input samples were mutated: %#v", columns[0].Samples)
	}
	if !samplesSorted(got) {
		t.Fatal("samplesSorted(sorted) = false, want true")
	}
}

func TestCollectTimestampsAlignedAndSparseColumns(t *testing.T) {
	aligned := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 20},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 20},
			},
		},
	}
	got := collectTimestamps(aligned)
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("aligned timestamps = %v, want [10 20]", got)
	}

	sparse := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 20},
				{Timestamp: 40},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 40},
			},
		},
	}
	got = collectTimestamps(sparse)
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 40 {
		t.Fatalf("sparse timestamps = %v, want [10 20 40]", got)
	}
}

func TestCollectTimestampsSparseOrderedAllocations(t *testing.T) {
	columns := []model.ColumnData{
		columnWithTimestamps(1, 1, 0, 100),
		columnWithTimestamps(1, 2, 50, 100),
		columnWithTimestamps(1, 3, 100, 100),
	}
	allocs := testing.AllocsPerRun(100, func() {
		got := collectTimestamps(columns)
		if len(got) != 200 {
			t.Fatalf("timestamp count = %d, want 200", len(got))
		}
		if got[0] != 0 || got[len(got)-1] != 199 {
			t.Fatalf("timestamp bounds = (%d,%d), want (0,199)", got[0], got[len(got)-1])
		}
	})
	if allocs > 8 {
		t.Fatalf("collectTimestamps ordered allocs/run = %.2f, want <= 8", allocs)
	}
}

func TestCollectTimestampsUnsortedFallbackAndSortedSeriesIDs(t *testing.T) {
	columns := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 30},
				{Timestamp: 10},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 20},
				{Timestamp: 10},
			},
		},
	}
	got := collectTimestamps(columns)
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("timestamps = %v, want [10 20 30]", got)
	}
	grouped := map[uint64][]model.ColumnData{
		3: nil,
		1: nil,
		2: nil,
	}
	ids := sortedSeriesIDs(grouped)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("series ids = %v, want [1 2 3]", ids)
	}
}

func columnWithTimestamps(seriesID uint64, fieldID uint32, start int64, count int) model.ColumnData {
	samples := make([]model.VersionedSample, 0, count)
	for index := 0; index < count; index++ {
		timestamp := start + int64(index)
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(timestamp)),
		})
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}
}

func TestPartQueryPrunesValueBlocksByField(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithField(1, 1, model.Float64Value(1)),
		columnWithField(1, 2, model.Int64Value(2)),
		columnWithField(1, 3, model.StringValue("skip")),
	}
	meta, err := WritePart(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := part.resetReadStatsForTest()
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       10,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || got[0].FieldID != 2 {
		t.Fatalf("Query() = %#v, want only field 2", got)
	}
	if stats.ValueBlocksRead != 1 {
		t.Fatalf("ValueBlocksRead = %d, want 1", stats.ValueBlocksRead)
	}
}

func TestOpenPartQueriesWithAlreadyOpenedBlockFiles(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}

	for _, name := range []string{indexFile, timestampsFile, valuesFile} {
		path := filepath.Join(meta.Path, name)
		if err := os.Rename(path, path+".moved"); err != nil {
			t.Fatalf("Rename(%s) error = %v", name, err)
		}
	}

	got, err := part.Query(Query{Start: 0, End: 10})
	if err != nil {
		t.Fatalf("Query() after block files moved error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 1 || got[0].Samples[0].Value.Float64 != 42 {
		t.Fatalf("Query() = %#v, want retained open file data", got)
	}
}

func TestPartCloseIsIdempotentAndNilSafe(t *testing.T) {
	if err := (*Part)(nil).Close(); err != nil {
		t.Fatalf("nil Part Close() error = %v", err)
	}

	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(1)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
}

func TestOpenPartReadFilesReportsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing index) error = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, indexFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing timestamps) error = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, timestampsFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(timestamps) error = %v", err)
	}
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing values) error = nil, want error")
	}
}

func TestOpenPartClosesReadFilesOnBadMetaIndex(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{indexFile, timestampsFile, valuesFile} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	metaIndexRef, err := writeBinaryBlock(filepath.Join(dir, metaindexFile), []byte("{"))
	if err != nil {
		t.Fatalf("writeBinaryBlock(metaindex) error = %v", err)
	}
	meta := metadata{
		FormatVersion: partFormatVersion,
		Part:          PartMeta{ID: "bad-metaindex"},
		MetaIndexRef:  metaIndexRef,
	}
	if err := writeMetadata(dir, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	if _, err := OpenPart(dir); err == nil {
		t.Fatal("OpenPart(bad metaindex) error = nil, want error")
	}
}

func TestPartReadBlockRejectsUnknownFile(t *testing.T) {
	part := &Part{files: &partReadFiles{}}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(unknown file) error = nil, want error")
	}
	if err := closeFile(nil, "nil"); err != nil {
		t.Fatalf("closeFile(nil) error = %v", err)
	}
}

func columnWithField(seriesID uint64, fieldID uint32, value model.FieldValue) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: value.Type,
		Samples: []model.VersionedSample{
			{Timestamp: 5, WriteSeq: 1, Value: value},
		},
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
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, nil, Query{Start: 0, End: 10})
	if err == nil {
		t.Fatal("readValueColumn(bad json) error = nil, want error")
	}
	missingPart := &Part{path: filepath.Join(dir, "missing")}
	if _, err := missingPart.readTimeBlock(blockRef{}); err == nil {
		t.Fatal("readTimeBlock(missing file) error = nil, want error")
	}
	filtered := columnFromBlock(1, filterValueBlock(valueBlock{
		FieldID:   2,
		FieldType: model.FieldInt64,
		Samples: []model.VersionedSample{
			{Timestamp: 100, WriteSeq: 1, Value: model.Int64Value(1)},
		},
	}, Query{Start: 0, End: 10}))
	if len(filtered.Samples) != 0 {
		t.Fatalf("filtered sample count = %d, want 0", len(filtered.Samples))
	}
}

func TestBinaryEncodingValidationErrors(t *testing.T) {
	if _, err := unmarshalTimeBlock([]byte{99, 0}); err == nil {
		t.Fatal("unmarshalTimeBlock(unknown) error = nil, want error")
	}
	if _, err := unmarshalValueBlock([]byte{99, 0}); err == nil {
		t.Fatal("unmarshalValueBlock(unknown) error = nil, want error")
	}
	if _, err := marshalValueBlock(nil, model.ColumnData{
		FieldType: model.FieldType(99),
		Samples:   []model.VersionedSample{{Timestamp: 1, Value: model.FieldValue{Type: model.FieldType(99)}}},
	}); err == nil {
		t.Fatal("marshalValueBlock(unknown) error = nil, want error")
	}
	reader := newBlockReader([]byte{1})
	if err := reader.done("sstable test"); err == nil {
		t.Fatal("blockReader.done(trailing) error = nil, want error")
	}
	if _, err := newBlockReader([]byte{0xff, 0xff, 0xff, 0xff, 0x1f}).uint32("overflow"); err == nil {
		t.Fatal("blockReader.uint32(overflow) error = nil, want error")
	}
	if _, err := readBlockRef(newBlockReader([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1})); err == nil {
		t.Fatal("readBlockRef(overflow) error = nil, want error")
	}
	if _, err := appendBlockRef(nil, blockRef{Offset: -1}); err == nil {
		t.Fatal("appendBlockRef(negative) error = nil, want error")
	}
}

func TestManifestNormalizeAndLegacyErrors(t *testing.T) {
	manifest := normalizeManifest(Manifest{Parts: []PartMeta{
		{ID: "b", Level: 1},
		{ID: "a", Level: 0},
	}})
	if manifest.Parts[0].ID != "a" {
		t.Fatalf("first manifest part = %q, want a", manifest.Parts[0].ID)
	}
	if normalizeManifest(Manifest{}).Parts == nil {
		t.Fatal("normalizeManifest() Parts = nil, want empty slice")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, legacyManifestFile), []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile(legacy manifest) error = %v", err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("LoadManifest(legacy) error = nil, want error")
	}
}

func TestMetadataEncodingValidationErrors(t *testing.T) {
	if _, err := encodeMetadata(metadata{Part: PartMeta{RowsCount: -1}}); err == nil {
		t.Fatal("encodeMetadata(negative count) error = nil, want error")
	}
	if _, err := encodeMetadata(metadata{IndexRef: blockRef{Offset: -1}}); err == nil {
		t.Fatal("encodeMetadata(negative ref) error = nil, want error")
	}
	if _, err := encodeIndexRows([]indexRow{{TimeRef: blockRef{Size: -1}}}); err == nil {
		t.Fatal("encodeIndexRows(negative ref) error = nil, want error")
	}
	if _, err := encodeMetaIndexRows([]metaIndexRow{{IndexRef: blockRef{Offset: -1}}}); err == nil {
		t.Fatal("encodeMetaIndexRows(negative ref) error = nil, want error")
	}
	if _, err := decodeMetadata([]byte{1}); err == nil {
		t.Fatal("decodeMetadata(short) error = nil, want error")
	}
	if _, err := decodeIndexRows([]byte{1}); err == nil {
		t.Fatal("decodeIndexRows(short) error = nil, want error")
	}
	if _, err := decodeMetaIndexRows([]byte{1}); err == nil {
		t.Fatal("decodeMetaIndexRows(short) error = nil, want error")
	}
	block := timeBlockFrom([]int64{1, 2})
	if block.Encoding != "plain-int64-v1" || block.MinTime != 1 || block.MaxTime != 2 {
		t.Fatalf("timeBlockFrom() = %#v, want legacy metadata fields", block)
	}
}

func TestSSTableBinaryDecodersRejectTruncatedPrefixes(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		FormatVersion: partFormatVersion,
		Part:          PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:      blockRef{Offset: 1, Size: 2},
		MetaIndexRef:  blockRef{Offset: 3, Size: 4},
	})
	if err != nil {
		t.Fatalf("encodeMetadata() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, metaPayload, func(data []byte) error {
		_, err := decodeMetadata(data)
		return err
	})

	indexPayload, err := encodeIndexRows([]indexRow{{
		SeriesID: 1,
		MinTime:  1,
		MaxTime:  2,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns:  []columnRef{{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 3, Size: 4}}},
	}})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, indexPayload, func(data []byte) error {
		_, err := decodeIndexRows(data)
		return err
	})

	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{{
		MinSeriesID: 1,
		MaxSeriesID: 2,
		MinTime:     1,
		MaxTime:     2,
		FieldIDs:    []uint32{1, 2},
		IndexRef:    blockRef{Offset: 1, Size: 2},
	}})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, metaIndexPayload, func(data []byte) error {
		_, err := decodeMetaIndexRows(data)
		return err
	})

	timePayload := marshalTimeBlock(nil, []int64{1, 2, 4})
	assertDecoderRejectsPrefixes(t, timePayload, func(data []byte) error {
		_, err := unmarshalTimeBlock(data)
		return err
	})

	valuePayload, err := marshalValueBlock(nil, columnWithField(1, 1, model.StringValue("abc")))
	if err != nil {
		t.Fatalf("marshalValueBlock() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, valuePayload, func(data []byte) error {
		_, err := unmarshalValueBlock(data)
		return err
	})
}

func TestSSTableEnvelopePayloadDecodersRejectTruncatedInnerPayload(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		FormatVersion: partFormatVersion,
		Part:          PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:      blockRef{Offset: 1, Size: 2},
		MetaIndexRef:  blockRef{Offset: 3, Size: 4},
	})
	if err != nil {
		t.Fatalf("encodeMetadata() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, metaPayload, partMagic, func(data []byte) error {
		_, err := decodeMetadata(data)
		return err
	})

	indexPayload, err := encodeIndexRows([]indexRow{{
		SeriesID: 1,
		MinTime:  1,
		MaxTime:  2,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns:  []columnRef{{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 3, Size: 4}}},
	}})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, indexPayload, indexMagic, func(data []byte) error {
		_, err := decodeIndexRows(data)
		return err
	})

	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{{
		MinSeriesID: 1,
		MaxSeriesID: 2,
		MinTime:     1,
		MaxTime:     2,
		FieldIDs:    []uint32{1, 2},
		IndexRef:    blockRef{Offset: 1, Size: 2},
	}})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, metaIndexPayload, metaIndexMagic, func(data []byte) error {
		_, err := decodeMetaIndexRows(data)
		return err
	})
}

func assertEnvelopePayloadPrefixes(t *testing.T, frame []byte, magic codec.Magic, decode func([]byte) error) {
	t.Helper()
	env, err := codec.UnmarshalEnvelope(frame, magic, partFormatVersion)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	for size := 0; size < len(env.Payload); size++ {
		prefixFrame := codec.MarshalEnvelope(nil, magic, partFormatVersion, 0, env.Payload[:size])
		if err := decode(prefixFrame); err == nil {
			t.Fatalf("decode(inner prefix %d/%d) error = nil, want error", size, len(env.Payload))
		}
	}
}

func assertDecoderRejectsPrefixes(t *testing.T, payload []byte, decode func([]byte) error) {
	t.Helper()
	for size := 0; size < len(payload); size++ {
		if err := decode(payload[:size]); err == nil {
			t.Fatalf("decode(prefix %d/%d) error = nil, want error", size, len(payload))
		}
	}
}

func TestPartQueryRejectsBadLazyIndexBlock(t *testing.T) {
	dir := t.TempDir()
	indexFileHandle := mustCreateTestFile(t, filepath.Join(dir, indexFile))
	indexRef, err := writeBlock(indexFileHandle, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(index) error = %v", err)
	}
	if err := indexFileHandle.Close(); err != nil {
		t.Fatalf("Close(index) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, timestampsFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(timestamps) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, valuesFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(values) error = %v", err)
	}
	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{
		{
			MinSeriesID: 1,
			MaxSeriesID: 1,
			MinTime:     0,
			MaxTime:     10,
			FieldIDs:    []uint32{1},
			IndexRef:    indexRef,
		},
	})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	metaIndexRef, err := writeBinaryBlock(filepath.Join(dir, metaindexFile), metaIndexPayload)
	if err != nil {
		t.Fatalf("writeBinaryBlock(metaindex) error = %v", err)
	}
	meta := metadata{
		FormatVersion: partFormatVersion,
		Part:          PartMeta{ID: "bad-index", MinTime: 0, MaxTime: 10, MinSeriesID: 1, MaxSeriesID: 1},
		IndexRef:      indexRef,
		MetaIndexRef:  metaIndexRef,
	}
	if err := writeMetadata(dir, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	part, err := OpenPart(dir)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if _, err := part.Query(Query{Start: 0, End: 10}); err == nil {
		t.Fatal("Query(bad lazy index block) error = nil, want error")
	}
}

func TestPartWriterPathErrors(t *testing.T) {
	if _, err := writeBinaryBlock(filepath.Join("bad\x00path", "index.bin"), []byte{1}); err == nil {
		t.Fatal("writeBinaryBlock(invalid) error = nil, want error")
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

func TestWritePartPropagatesUnsupportedValueType(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldType(99),
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.FieldValue{Type: model.FieldType(99)}},
			},
		},
	}
	if _, err := WritePart(t.TempDir(), 0, "sst-bad-type", columns); err == nil {
		t.Fatal("WritePart(unsupported type) error = nil, want encode error")
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
