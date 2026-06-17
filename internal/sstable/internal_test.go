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

func TestPartWriterAddsSeriesIncrementally(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewPartWriter(dir, 1, "sst-stream", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter() error = %v", err)
	}
	if err := writer.AddSeries([]model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldString,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.StringValue("ok")},
			},
		},
	}); err != nil {
		abortErr := writer.Abort()
		t.Fatalf("AddSeries(series 1) error = %v abort = %v", err, abortErr)
	}
	if err := writer.AddSeries([]model.ColumnData{
		{
			SeriesID:  2,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 30, WriteSeq: 3, Value: model.Float64Value(3)},
			},
		},
	}); err != nil {
		abortErr := writer.Abort()
		t.Fatalf("AddSeries(series 2) error = %v abort = %v", err, abortErr)
	}
	meta, err := writer.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if meta.Level != 1 || meta.SeriesCount != 2 || meta.RowsCount != 4 {
		t.Fatalf("meta = %#v, want level 1, 2 series, 4 rows", meta)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close(part) error = %v", err)
		}
	}()
	columns, err := part.Query(Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(columns) != 3 {
		t.Fatalf("column count = %d, want 3", len(columns))
	}
	seriesIDs, err := part.SeriesIDs(Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("SeriesIDs() error = %v", err)
	}
	if len(seriesIDs) != 2 || seriesIDs[0] != 1 || seriesIDs[1] != 2 {
		t.Fatalf("SeriesIDs() = %v, want [1 2]", seriesIDs)
	}
	if _, err := writer.Close(); err == nil {
		t.Fatal("Close(already closed) error = nil, want error")
	}
	if err := writer.AddSeries([]model.ColumnData{streamTestColumn(3, 1, 30)}); err == nil {
		t.Fatal("AddSeries(already closed) error = nil, want error")
	}
}

func TestPartWriterAbortAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewPartWriter(dir, 1, "sst-abort", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(abort) error = %v", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sst-abort")); !os.IsNotExist(err) {
		t.Fatalf("aborted part stat error = %v, want not exist", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("second Abort() error = %v", err)
	}
	if err := writer.AddSeries([]model.ColumnData{streamTestColumn(1, 1, 10)}); err == nil {
		t.Fatal("AddSeries(closed) error = nil, want error")
	}

	emptyWriter, err := NewPartWriter(dir, 1, "sst-empty-stream", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(empty) error = %v", err)
	}
	if _, err := emptyWriter.Close(); err == nil {
		t.Fatal("Close(empty writer) error = nil, want error")
	}
	if _, err := os.Stat(filepath.Join(dir, "sst-empty-stream")); !os.IsNotExist(err) {
		t.Fatalf("empty closed part stat error = %v, want not exist", err)
	}

	badWriter, err := NewPartWriter(dir, 1, "sst-bad-series", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(bad series) error = %v", err)
	}
	if err := badWriter.AddSeries([]model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 10),
	}); err == nil {
		abortErr := badWriter.Abort()
		t.Fatalf("AddSeries(mixed series) error = nil, want error abort = %v", abortErr)
	}
	if err := badWriter.Abort(); err != nil {
		t.Fatalf("Abort(bad series) error = %v", err)
	}

	emptyColumnWriter, err := NewPartWriter(dir, 1, "sst-empty-column", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(empty column) error = %v", err)
	}
	if err := emptyColumnWriter.AddSeries([]model.ColumnData{
		{SeriesID: 1, FieldID: 1, FieldType: model.FieldFloat64},
	}); err == nil {
		abortErr := emptyColumnWriter.Abort()
		t.Fatalf("AddSeries(empty column) error = nil, want error abort = %v", abortErr)
	}
	if err := emptyColumnWriter.Abort(); err != nil {
		t.Fatalf("Abort(empty column) error = %v", err)
	}

	cleanupWriter, err := NewPartWriter(dir, 1, "sst-close-cleanup", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(cleanup) error = %v", err)
	}
	if err := cleanupWriter.AddSeries([]model.ColumnData{streamTestColumn(1, 1, 10)}); err != nil {
		abortErr := cleanupWriter.Abort()
		t.Fatalf("AddSeries(cleanup) error = %v abort = %v", err, abortErr)
	}
	cleanupPath := filepath.Join(dir, "sst-close-cleanup")
	if err := os.Mkdir(filepath.Join(cleanupPath, indexFile), 0700); err != nil {
		abortErr := cleanupWriter.Abort()
		t.Fatalf("Mkdir(index collision) error = %v abort = %v", err, abortErr)
	}
	if _, err := cleanupWriter.Close(); err == nil {
		t.Fatal("Close(index collision) error = nil, want error")
	}
	if _, err := os.Stat(cleanupPath); !os.IsNotExist(err) {
		t.Fatalf("cleanup part stat error = %v, want not exist", err)
	}
}

func TestPartWriterRejectsInvalidRoot(t *testing.T) {
	if _, err := NewPartWriter("bad\x00path", 1, "sst-bad", WriteOptions{}); err == nil {
		t.Fatal("NewPartWriter(invalid root) error = nil, want error")
	}
}

func TestPartSeriesIDsAndUnsupportedBlockFileBranches(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-ids", []model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 20),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	ids, err := part.SeriesIDs(Query{Start: 100, End: 10})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(empty range) error = %v close = %v", err, closeErr)
	}
	if len(ids) != 0 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(empty range) = %v, want none close = %v", ids, closeErr)
	}
	ids, err = part.SeriesIDs(Query{
		Start:     0,
		End:       100,
		SeriesIDs: map[uint64]struct{}{99: {}},
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(filtered) error = %v close = %v", err, closeErr)
	}
	if len(ids) != 0 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(filtered) = %v, want none close = %v", ids, closeErr)
	}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		closeErr := part.Close()
		t.Fatalf("readBlock(unsupported) error = nil, want error close = %v", closeErr)
	}
	if _, err := part.readBlockPayload("unknown.bin", blockRef{}); err == nil {
		closeErr := part.Close()
		t.Fatalf("readBlockPayload(unsupported) error = nil, want error close = %v", closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	closedPart := &Part{path: meta.Path}
	if _, err := closedPart.readBlock("missing.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(closed missing file) error = nil, want error")
	}
	if _, err := closedPart.readBlockPayload("missing.bin", blockRef{}); err == nil {
		t.Fatal("readBlockPayload(closed missing file) error = nil, want error")
	}
}

func TestIndexRowStreamSkipsAndReportsUndrainedRows(t *testing.T) {
	encoded, err := encodeIndexRows([]indexRow{
		{
			SeriesID: 1,
			MinTime:  10,
			MaxTime:  20,
			Columns: []columnRef{
				{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 1, Size: 2}},
			},
		},
	})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	stream, err := newIndexRowStream(encoded)
	if err != nil {
		t.Fatalf("newIndexRowStream() error = %v", err)
	}
	if err := stream.done(); err == nil {
		t.Fatal("done(undrained) error = nil, want error")
	}
	header, ok, err := stream.nextHeader()
	if err != nil {
		t.Fatalf("nextHeader() error = %v", err)
	}
	if !ok || header.seriesID != 1 {
		t.Fatalf("nextHeader() = %#v ok=%v, want series 1", header, ok)
	}
	if err := stream.skipColumnRefs(); err != nil {
		t.Fatalf("skipColumnRefs() error = %v", err)
	}
	if _, ok, err := stream.nextHeader(); err != nil || ok {
		t.Fatalf("nextHeader(done) ok=%v err=%v, want false nil", ok, err)
	}
	if err := stream.done(); err != nil {
		t.Fatalf("done() error = %v", err)
	}
}

func TestSeriesBatchReaderCachesIndexRows(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-reader", []model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 20),
		streamTestColumn(3, 1, 30),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	reader, err := NewSeriesBatchReader(part, Query{Start: 0, End: 100})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("NewSeriesBatchReader() error = %v close = %v", err, closeErr)
	}
	ids := reader.SeriesIDs()
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs() = %v, want [1 2 3] close = %v", ids, closeErr)
	}
	ids[0] = 99
	if reader.SeriesIDs()[0] != 1 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs() exposed internal slice close = %v", closeErr)
	}
	if reader.SeriesCount() != 3 {
		closeErr := part.Close()
		t.Fatalf("SeriesCount() = %d, want 3 close = %v", reader.SeriesCount(), closeErr)
	}
	appended := reader.AppendSeriesIDs([]uint64{0})
	if len(appended) != 4 || appended[0] != 0 || appended[1] != 1 || appended[3] != 3 {
		closeErr := part.Close()
		t.Fatalf("AppendSeriesIDs() = %v, want [0 1 2 3] close = %v", appended, closeErr)
	}
	direct, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{1, 3})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs() error = %v close = %v", err, closeErr)
	}
	if len(direct) != 2 || direct[0].SeriesID != 1 || direct[1].SeriesID != 3 {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs() = %#v, want series 1 and 3 close = %v", direct, closeErr)
	}
	directEmpty, err := part.QuerySeriesIDs(Query{Start: 100, End: 10}, []uint64{1})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(empty range) error = %v close = %v", err, closeErr)
	}
	if len(directEmpty) != 0 {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(empty range) = %d columns, want 0 close = %v", len(directEmpty), closeErr)
	}
	filteredMeta, err := WritePart(dir, 0, "sst-series-reader-fields", []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(2)},
			},
		},
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("WritePart(filtered fields) error = %v close = %v", err, closeErr)
	}
	filteredPart, err := OpenPart(filteredMeta.Path)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("OpenPart(filtered fields) error = %v close = %v", err, closeErr)
	}
	stats := filteredPart.resetReadStatsForTest()
	fieldColumns, err := filteredPart.QuerySeriesIDs(Query{
		FieldIDs: map[uint32]struct{}{2: {}},
		Start:    0,
		End:      100,
	}, []uint64{1})
	if err != nil {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(filtered fields) error = %v filtered close = %v close = %v", err, filteredCloseErr, closeErr)
	}
	if len(fieldColumns) != 1 || fieldColumns[0].FieldID != 2 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(filtered fields) = %#v, want only field 2 filtered close = %v close = %v", fieldColumns, filteredCloseErr, closeErr)
	}
	if stats.TimeBlocksRead != 1 || stats.ValueBlocksRead != 1 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("read stats = %#v, want one time block and one value block filtered close = %v close = %v", stats, filteredCloseErr, closeErr)
	}
	stats = filteredPart.resetReadStatsForTest()
	missingFieldColumns, err := filteredPart.QuerySeriesIDs(Query{
		FieldIDs: map[uint32]struct{}{99: {}},
		Start:    0,
		End:      100,
	}, []uint64{1})
	if err != nil {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(missing field) error = %v filtered close = %v close = %v", err, filteredCloseErr, closeErr)
	}
	if len(missingFieldColumns) != 0 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(missing field) = %#v, want none filtered close = %v close = %v", missingFieldColumns, filteredCloseErr, closeErr)
	}
	if stats.TimeBlocksRead != 0 || stats.ValueBlocksRead != 0 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("missing field read stats = %#v, want no data block reads filtered close = %v close = %v", stats, filteredCloseErr, closeErr)
	}
	if err := filteredPart.Close(); err != nil {
		closeErr := part.Close()
		t.Fatalf("Close(filteredPart) error = %v close = %v", err, closeErr)
	}
	if err := os.WriteFile(filepath.Join(meta.Path, indexFile), []byte{0xff}, 0600); err != nil {
		closeErr := part.Close()
		t.Fatalf("WriteFile(corrupt index) error = %v close = %v", err, closeErr)
	}
	if _, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{2}); err == nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(corrupt index) error = nil, want error close = %v", closeErr)
	}
	columns, err := reader.QuerySeriesIDs([]uint64{2, 3})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 2 || columns[0].SeriesID != 2 || columns[1].SeriesID != 3 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs() = %#v, want series 2 and 3 close = %v", columns, closeErr)
	}
	single, err := reader.QuerySeriesID(2)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID() error = %v close = %v", err, closeErr)
	}
	if len(single) != 1 || single[0].SeriesID != 2 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID() = %#v, want only series 2 close = %v", single, closeErr)
	}
	missingSingle, err := reader.QuerySeriesID(99)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID(missing) error = %v close = %v", err, closeErr)
	}
	if len(missingSingle) != 0 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID(missing) = %#v, want none close = %v", missingSingle, closeErr)
	}
	empty, err := reader.QuerySeriesIDs(nil)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(nil) error = %v close = %v", err, closeErr)
	}
	if len(empty) != 0 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(nil) len = %d, want 0 close = %v", len(empty), closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	emptyReader, err := NewSeriesBatchReader(part, Query{Start: 100, End: 10})
	if err != nil {
		t.Fatalf("NewSeriesBatchReader(empty range) error = %v", err)
	}
	if emptyReader.SeriesCount() != 0 {
		t.Fatalf("empty SeriesCount() = %d, want 0", emptyReader.SeriesCount())
	}
	if got := emptyReader.AppendSeriesIDs(nil); len(got) != 0 {
		t.Fatalf("empty AppendSeriesIDs() = %v, want none", got)
	}
	var nilReader *SeriesBatchReader
	if nilReader.SeriesCount() != 0 {
		t.Fatalf("nil SeriesCount() = %d, want 0", nilReader.SeriesCount())
	}
	if got := nilReader.AppendSeriesIDs([]uint64{7}); len(got) != 1 || got[0] != 7 {
		t.Fatalf("nil AppendSeriesIDs() = %v, want [7]", got)
	}
	if got, err := nilReader.QuerySeriesID(1); err != nil || len(got) != 0 {
		t.Fatalf("nil QuerySeriesID() = %#v err = %v, want empty nil", got, err)
	}
}

func streamTestColumn(seriesID uint64, fieldID uint32, timestamp int64) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: timestamp, WriteSeq: 1, Value: model.Float64Value(float64(timestamp))},
		},
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

func TestPartQueryReadsOnlyMatchingValuePages(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithTimestamps(1, 2, 0, valueBlockPageSamples*3),
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
		Start:     int64(valueBlockPageSamples + 10),
		End:       int64(valueBlockPageSamples + 10),
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 1 {
		t.Fatalf("query result = %#v, want one sample", got)
	}
	if stats.ValuePagesRead != 1 {
		t.Fatalf("value pages read = %d, want 1", stats.ValuePagesRead)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPartReadBlockRejectsUnsupportedOpenFileName(t *testing.T) {
	part := &Part{files: &partReadFiles{}}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(unknown) error = nil, want error")
	}
	if _, err := part.readBlockPayload("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlockPayload(unknown) error = nil, want error")
	}
}

func TestPartReadBlockUsesOpenFiles(t *testing.T) {
	dir := t.TempDir()
	indexFileHandle, indexRef := writeSingleBlockFileForTest(t, filepath.Join(dir, indexFile), []byte("index"))
	timeFileHandle, timeRef := writeSingleBlockFileForTest(t, filepath.Join(dir, timestampsFile), []byte("time"))
	valueFileHandle, valueRef := writeSingleBlockFileForTest(t, filepath.Join(dir, valuesFile), []byte("value"))
	part := &Part{files: &partReadFiles{
		index:      indexFileHandle,
		timestamps: timeFileHandle,
		values:     valueFileHandle,
	}}
	tests := []struct {
		name string
		file string
		ref  blockRef
		want string
	}{
		{name: "index", file: indexFile, ref: indexRef, want: "index"},
		{name: "timestamps", file: timestampsFile, ref: timeRef, want: "time"},
		{name: "values", file: valuesFile, ref: valueRef, want: "value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := part.readBlock(tt.file, tt.ref)
			if err != nil {
				t.Fatalf("readBlock(%s) error = %v", tt.file, err)
			}
			if string(got) != tt.want {
				t.Fatalf("readBlock(%s) = %q, want %q", tt.file, string(got), tt.want)
			}
		})
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func writeSingleBlockFileForTest(t *testing.T, path string, payload []byte) (*os.File, blockRef) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", path, err)
	}
	ref, err := writeBlock(file, payload)
	if err != nil {
		closeErr := file.Close()
		t.Fatalf("writeBlock(%s) error = %v close = %v", path, err, closeErr)
	}
	if _, err := file.Seek(0, 0); err != nil {
		closeErr := file.Close()
		t.Fatalf("Seek(%s) error = %v close = %v", path, err, closeErr)
	}
	return file, ref
}

func TestWritePartWithCompressionOptionsRoundTrips(t *testing.T) {
	dir := t.TempDir()
	column := columnWithTimestamps(1, 2, 0, 64)
	meta, err := WritePartWithOptions(dir, 0, "sst-000001", []model.ColumnData{column}, WriteOptions{
		Compression: model.CompressionOptions{Enabled: true, MinPageValues: 1},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     10,
		End:       12,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 3 {
		t.Fatalf("compressed query result = %#v, want one column with 3 samples", got)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWritePartWithPayloadCompressionAlgorithmsRoundTrip(t *testing.T) {
	algorithms := []string{"snappy", "lz4", "zstd"}
	for _, algorithm := range algorithms {
		t.Run(algorithm, func(t *testing.T) {
			dir := t.TempDir()
			columns := payloadCompressionColumns()
			meta, err := WritePartWithOptions(dir, 0, "sst-000001", columns, WriteOptions{
				Compression: model.CompressionOptions{
					Enabled:       true,
					MinPageValues: 1,
					Algorithm:     algorithm,
				},
			})
			if err != nil {
				t.Fatalf("WritePartWithOptions(%s) error = %v", algorithm, err)
			}
			part, err := OpenPart(meta.Path)
			if err != nil {
				t.Fatalf("OpenPart(%s) error = %v", algorithm, err)
			}
			got, err := part.Query(Query{Start: 10, End: 12})
			if err != nil {
				closeErr := part.Close()
				t.Fatalf("Query(%s) error = %v close = %v", algorithm, err, closeErr)
			}
			if err := part.Close(); err != nil {
				t.Fatalf("Close(%s) error = %v", algorithm, err)
			}
			assertPayloadCompressionQuery(t, got)
		})
	}
}

func TestWritePartWithPayloadCompressionReducesValuesFileSize(t *testing.T) {
	columns := payloadCompressionColumns()
	plainDir := t.TempDir()
	plainMeta, err := WritePartWithOptions(plainDir, 0, "sst-plain", columns, WriteOptions{
		Compression: model.CompressionOptions{Enabled: true, MinPageValues: 1},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions(plain) error = %v", err)
	}
	zstdDir := t.TempDir()
	zstdMeta, err := WritePartWithOptions(zstdDir, 0, "sst-zstd", columns, WriteOptions{
		Compression: model.CompressionOptions{
			Enabled:       true,
			MinPageValues: 1,
			Algorithm:     "zstd",
		},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions(zstd) error = %v", err)
	}
	plainSize := fileSizeForTest(t, filepath.Join(plainMeta.Path, valuesFile))
	zstdSize := fileSizeForTest(t, filepath.Join(zstdMeta.Path, valuesFile))
	if zstdSize >= plainSize {
		t.Fatalf("zstd values.bin size = %d, want smaller than plain %d", zstdSize, plainSize)
	}
}

func payloadCompressionColumns() []model.ColumnData {
	count := 512
	return []model.ColumnData{
		payloadCompressionColumn(1, 1, model.FieldFloat64, count, func(index int) model.FieldValue {
			return model.Float64Value(42.25)
		}),
		payloadCompressionColumn(1, 2, model.FieldInt64, count, func(index int) model.FieldValue {
			return model.Int64Value(1_000_000 + int64(index%4))
		}),
		payloadCompressionColumn(1, 3, model.FieldString, count, func(index int) model.FieldValue {
			return model.StringValue("payload-compression-value-payload-compression-value")
		}),
		payloadCompressionColumn(1, 4, model.FieldBool, count, func(index int) model.FieldValue {
			return model.BoolValue(index%2 == 0)
		}),
	}
}

func payloadCompressionColumn(
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
	count int,
	value func(index int) model.FieldValue,
) model.ColumnData {
	samples := make([]model.VersionedSample, count)
	for index := range count {
		samples[index] = model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     value(index),
		}
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: fieldType,
		Samples:   samples,
	}
}

func assertPayloadCompressionQuery(t *testing.T, got []model.ColumnData) {
	t.Helper()
	if len(got) != 4 {
		t.Fatalf("column count = %d, want 4", len(got))
	}
	for _, column := range got {
		if len(column.Samples) != 3 {
			t.Fatalf("field %d sample count = %d, want 3", column.FieldID, len(column.Samples))
		}
		if column.Samples[0].Timestamp != 10 || column.Samples[2].Timestamp != 12 {
			t.Fatalf("field %d samples = %#v, want timestamps 10..12", column.FieldID, column.Samples)
		}
	}
}

func fileSizeForTest(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
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

func TestPartQueryFallsBackToPathAfterClose(t *testing.T) {
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
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	got, err := part.Query(Query{Start: 0, End: 10})
	if err != nil {
		t.Fatalf("Query() after Close error = %v", err)
	}
	if len(got) != 1 || got[0].Samples[0].Value.Float64 != 42 {
		t.Fatalf("Query() after Close = %#v, want value 42", got)
	}
}

func TestSSTableSmallHelpersAndOpenPartFilesErrors(t *testing.T) {
	if valuePageCount(0) != 0 {
		t.Fatalf("valuePageCount(0) = %d, want 0", valuePageCount(0))
	}
	if _, err := uint32Value("too large", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	row := indexRow{
		SeriesID: 10,
		MinTime:  5,
		MaxTime:  10,
		Columns:  []columnRef{{FieldID: 2}},
	}
	if rowMatches(row, Query{SeriesIDs: map[uint64]struct{}{11: {}}, Start: 0, End: 20}) {
		t.Fatal("rowMatches(series mismatch) = true, want false")
	}
	if rowMatches(row, Query{Start: 20, End: 30}) {
		t.Fatal("rowMatches(time mismatch) = true, want false")
	}
	header := indexRowHeader{
		seriesID: 10,
		minTime:  5,
		maxTime:  10,
	}
	if rowHeaderMatches(header, Query{SeriesIDs: map[uint64]struct{}{11: {}}, Start: 0, End: 20}) {
		t.Fatal("rowHeaderMatches(series mismatch) = true, want false")
	}
	if rowHeaderMatches(header, Query{Start: 20, End: 30}) {
		t.Fatal("rowHeaderMatches(time mismatch) = true, want false")
	}
	if _, err := openPartFiles("bad\x00path"); err == nil {
		t.Fatal("openPartFiles(invalid) error = nil, want error")
	}
	pageHeader := valuePageIndexHeader{count: 10, pageCount: 2}
	if got := matchingValuePageCapacity(pageHeader, 0); got != 0 {
		t.Fatalf("matchingValuePageCapacity(no match) = %d, want 0", got)
	}
	if got := matchingValuePageCapacity(pageHeader, 1); got != 5 {
		t.Fatalf("matchingValuePageCapacity(one match) = %d, want 5", got)
	}
	pagePayload, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     10,
		Pages: []valuePageRef{
			{MinTime: 0, MaxTime: 4, Ref: blockRef{Offset: 1, Size: 2}},
			{MinTime: 5, MaxTime: 9, Ref: blockRef{Offset: 3, Size: 4}},
		},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	gotHeader, matches, err := matchingValuePageIndexHeader(pagePayload, Query{Start: 4, End: 6})
	if err != nil {
		t.Fatalf("matchingValuePageIndexHeader() error = %v", err)
	}
	if gotHeader.fieldID != 1 || gotHeader.pageCount != 2 || matches != 2 {
		t.Fatalf("page header = %#v matches=%d, want field 1 page count 2 matches 2", gotHeader, matches)
	}
	if _, _, err := matchingValuePageIndexHeader(pagePayload[:len(pagePayload)-1], Query{Start: 0, End: 10}); err == nil {
		t.Fatal("matchingValuePageIndexHeader(truncated) error = nil, want error")
	}
	column, err := (*Part)(nil).readValuePagesFromIndexPayload(7, pagePayload, nil, Query{Start: 20, End: 30})
	if err != nil {
		t.Fatalf("readValuePagesFromIndexPayload(no match) error = %v", err)
	}
	if column.SeriesID != 7 || column.FieldID != 1 || len(column.Samples) != 0 {
		t.Fatalf("no-match column = %#v, want empty series 7 field 1", column)
	}
}

func TestValuePageIndexFullRangeUsesSinglePass(t *testing.T) {
	pagePayload, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     12,
		Pages: []valuePageRef{
			{MinTime: 0, MaxTime: 3, Ref: blockRef{Offset: 1, Size: 2}},
			{MinTime: 4, MaxTime: 7, Ref: blockRef{Offset: 3, Size: 4}},
			{MinTime: 8, MaxTime: 11, Ref: blockRef{Offset: 5, Size: 6}},
		},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	reads := 0
	valuePageRefReadHook = func() {
		reads++
	}
	t.Cleanup(func() {
		valuePageRefReadHook = nil
	})
	header, pages, fullRange, matches, err := scanValuePageIndexCoverage(pagePayload, Query{Start: 0, End: 11})
	if err != nil {
		t.Fatalf("scanValuePageIndexCoverage() error = %v", err)
	}
	if !fullRange || matches != 3 || len(pages) != 3 {
		t.Fatalf("fullRange=%v matches=%d pages=%d, want true 3 3", fullRange, matches, len(pages))
	}
	if header.count != 12 {
		t.Fatalf("header count = %d, want 12", header.count)
	}
	if reads != 3 {
		t.Fatalf("value page refs read = %d, want 3", reads)
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
		Part:         PartMeta{ID: "bad-metaindex"},
		MetaIndexRef: metaIndexRef,
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
	if _, err := unmarshalValueBlockWithTimestamps([]byte{99, 0}, nil, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unknown) error = nil, want error")
	}
	if _, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldType: model.FieldType(99),
		Samples:   []model.VersionedSample{{Timestamp: 1, Value: model.FieldValue{Type: model.FieldType(99)}}},
	}, []int64{1}); err == nil {
		t.Fatal("marshalValueBlockWithTimestamps(unknown) error = nil, want error")
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

func TestManifestNormalizeAndMissingFile(t *testing.T) {
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
	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(missing) error = %v", err)
	}
	if loaded.Parts == nil || len(loaded.Parts) != 0 {
		t.Fatalf("LoadManifest(missing) parts = %#v, want empty slice", loaded.Parts)
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
	if block.Encoding != "plain-int64" || block.MinTime != 1 || block.MaxTime != 2 {
		t.Fatalf("timeBlockFrom() = %#v, want current metadata fields", block)
	}
}

func TestSSTableBinaryDecodersRejectTruncatedPrefixes(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		Part:         PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:     blockRef{Offset: 1, Size: 2},
		MetaIndexRef: blockRef{Offset: 3, Size: 4},
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

	valueColumn := columnWithField(1, 1, model.StringValue("abc"))
	valuePayload, err := marshalValueBlockWithTimestamps(nil, valueColumn, sampleTimestamps(valueColumn.Samples))
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, valuePayload, func(data []byte) error {
		_, err := unmarshalValueBlockWithTimestamps(data, sampleTimestamps(valueColumn.Samples), Query{Start: 0, End: 100})
		return err
	})
}

func TestSSTableEnvelopePayloadDecodersRejectTruncatedInnerPayload(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		Part:         PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:     blockRef{Offset: 1, Size: 2},
		MetaIndexRef: blockRef{Offset: 3, Size: 4},
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
	env, err := codec.UnmarshalEnvelope(frame, magic)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	for size := 0; size < len(env.Payload); size++ {
		prefixFrame := codec.MarshalEnvelope(nil, magic, 0, env.Payload[:size])
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
		Part:         PartMeta{ID: "bad-index", MinTime: 0, MaxTime: 10, MinSeriesID: 1, MaxSeriesID: 1},
		IndexRef:     indexRef,
		MetaIndexRef: metaIndexRef,
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
