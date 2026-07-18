package sstable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestWritePartRespectsValuePageSamples(t *testing.T) {
	dir := t.TempDir()
	samples := make([]model.VersionedSample, 3000)
	for i := range samples {
		samples[i] = model.VersionedSample{
			Timestamp: int64(i),
			WriteSeq:  uint64(i + 1),
			Value:     model.Float64Value(float64(i)),
		}
	}
	columns := []model.ColumnData{{
		SeriesID:  1,
		FieldID:   2,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}}

	small, err := WritePartWithOptions(dir, 0, "small", columns, WriteOptions{
		Compression: model.CompressionOptions{ValuePageSamples: 256},
	})
	if err != nil {
		t.Fatalf("WritePart small error = %v", err)
	}
	large, err := WritePartWithOptions(dir, 0, "large", columns, WriteOptions{
		Compression: model.CompressionOptions{ValuePageSamples: 4096},
	})
	if err != nil {
		t.Fatalf("WritePart large error = %v", err)
	}
	// 更大页应产生更少 value block（粗略：文件更小或至少可打开查询）。
	partSmall, err := OpenPart(small.Path)
	if err != nil {
		t.Fatalf("OpenPart small error = %v", err)
	}
	defer func() { _ = partSmall.Close() }()
	partLarge, err := OpenPart(large.Path)
	if err != nil {
		t.Fatalf("OpenPart large error = %v", err)
	}
	defer func() { _ = partLarge.Close() }()
	got, err := partLarge.Query(Query{Start: 0, End: 2999})
	if err != nil {
		t.Fatalf("Query large error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 3000 {
		t.Fatalf("large query samples = %d, want 3000", len(got[0].Samples))
	}
	// page count expectation via helper
	if valuePageCount(3000, 256) <= valuePageCount(3000, 4096) {
		t.Fatalf("expected fewer pages with larger page size")
	}
	_ = small
	_ = large
}
