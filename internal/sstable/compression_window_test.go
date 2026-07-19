package sstable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestReadCompressedSamplesConstStepWindow(t *testing.T) {
	count := 4096
	samples := make([]model.VersionedSample, count)
	for i := range samples {
		samples[i] = model.VersionedSample{
			Timestamp: int64(i) * 1_000_000_000,
			WriteSeq:  0,
			Value:     model.Float64Value(float64(i) * 1.1),
		}
	}
	column := model.ColumnData{
		SeriesID: 1, FieldID: 1, FieldType: model.FieldFloat64, Samples: samples,
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled: true, Algorithm: "none", MinPageValues: 1, OmitWriteSeq: true,
	}, nil)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	query := Query{Start: 1000 * 1_000_000_000, End: 1019 * 1_000_000_000}
	got, err := unmarshalCompressedValueBlock(payload, query)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(got.Samples) != 20 {
		t.Fatalf("samples=%d want 20", len(got.Samples))
	}
	if got.Samples[0].Timestamp != 1000*1_000_000_000 || got.Samples[0].Value.Float64 != 1000*1.1 {
		t.Fatalf("first sample = %#v", got.Samples[0])
	}
	if got.Samples[19].Timestamp != 1019*1_000_000_000 {
		t.Fatalf("last ts=%d", got.Samples[19].Timestamp)
	}
}
