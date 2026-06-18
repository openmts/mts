package sstable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestPartScanStatsRecordsSkippedPagesAndIndexRows(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-query-stats", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, valueBlockPageSamples*3),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := &model.QueryStats{}
	stream, err := part.ScanColumns(Query{
		Stats:     stats,
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     int64(valueBlockPageSamples + 10),
		End:       int64(valueBlockPageSamples + 10),
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("ScanColumns() error = %v close = %v", err, closeErr)
	}
	var columns int
	for stream.Next() {
		columns++
	}
	if err := stream.Err(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Close() error = %v part close = %v", err, closeErr)
	}
	if columns != 1 {
		closeErr := part.Close()
		t.Fatalf("columns = %d, want 1 close = %v", columns, closeErr)
	}
	if stats.PartsScanned != 1 || stats.PartsSkipped != 0 {
		closeErr := part.Close()
		t.Fatalf("part stats = %#v, want one scanned part close = %v", stats, closeErr)
	}
	if stats.IndexRowsRead != 1 {
		closeErr := part.Close()
		t.Fatalf("IndexRowsRead = %d, want 1 close = %v", stats.IndexRowsRead, closeErr)
	}
	if stats.ValuePagesRead != 1 || stats.ValuePagesSkipped != 2 {
		closeErr := part.Close()
		t.Fatalf("value page stats = %#v, want one read and two skipped close = %v", stats, closeErr)
	}
	if stats.SamplesRead != 1 {
		closeErr := part.Close()
		t.Fatalf("SamplesRead = %d, want 1 close = %v", stats.SamplesRead, closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestMetadataEmptyQueryDoesNotReadValues(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-query-stats-empty", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 10),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := &model.QueryStats{}
	stream, err := part.ScanColumns(Query{
		Stats:     stats,
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{99: {}},
		Start:     0,
		End:       10,
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("ScanColumns() error = %v close = %v", err, closeErr)
	}
	if stream.Next() {
		closeErr := part.Close()
		t.Fatalf("Next() = true, want empty metadata-filtered result close = %v", closeErr)
	}
	if err := stream.Err(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Close() error = %v part close = %v", err, closeErr)
	}
	if stats.PartsScanned != 0 || stats.PartsSkipped != 1 {
		closeErr := part.Close()
		t.Fatalf("part stats = %#v, want one metadata-skipped part close = %v", stats, closeErr)
	}
	if stats.ValueBlocksRead != 0 || stats.ValuePagesRead != 0 {
		closeErr := part.Close()
		t.Fatalf("value stats = %#v, want no value reads close = %v", stats, closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFirstLastBoundaryQueryReadsOnlyBoundaryValuePages(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-query-boundary", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, valueBlockPageSamples*3),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := &model.QueryStats{}
	stream, err := part.ScanColumns(Query{
		Stats:     stats,
		Boundary:  model.QueryBoundaryFirst,
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       int64(valueBlockPageSamples*3 - 1),
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("ScanColumns() error = %v close = %v", err, closeErr)
	}
	var samples int
	for stream.Next() {
		samples += len(stream.ColumnData().Samples)
	}
	if err := stream.Err(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		closeErr := part.Close()
		t.Fatalf("stream Close() error = %v part close = %v", err, closeErr)
	}
	if samples != valueBlockPageSamples {
		closeErr := part.Close()
		t.Fatalf("samples = %d, want one boundary page close = %v", samples, closeErr)
	}
	if stats.ValuePagesRead != 1 || stats.ValuePagesSkipped != 2 {
		closeErr := part.Close()
		t.Fatalf("value page stats = %#v, want one read and two skipped close = %v", stats, closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
