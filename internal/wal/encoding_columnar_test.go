package wal

import (
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestColumnarEncodeDecodeRoundTrip(t *testing.T) {
	records := []model.ResolvedPoint{
		{
			Database: "db", RetentionPolicy: "rp", Measurement: "cpu",
			Tags:     map[string]string{"host": "a"},
			SeriesID: 1, Timestamp: 10, WriteSeq: 1,
			Fields: []model.ResolvedField{
				{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(1.5)},
				{FieldID: 2, FieldName: "active", Type: model.FieldBool, Value: model.BoolValue(true)},
			},
		},
		{
			Database: "db", RetentionPolicy: "rp", Measurement: "cpu",
			Tags:     map[string]string{"host": "b"},
			SeriesID: 2, Timestamp: 20, WriteSeq: 2,
			Fields: []model.ResolvedField{
				{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(2.5)},
			},
		},
	}
	payload, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	got, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch() error = %v", err)
	}
	if !reflect.DeepEqual(got, records) {
		t.Fatalf("round-trip = %#v, want %#v", got, records)
	}
}

func TestColumnarWriteSeqRegressionEncoding(t *testing.T) {
	records := []model.ResolvedPoint{
		{SeriesID: 1, Timestamp: 1, WriteSeq: 10},
		{SeriesID: 1, Timestamp: 2, WriteSeq: 3},
		{SeriesID: 1, Timestamp: 3, WriteSeq: 4},
	}
	payload, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	got, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch() error = %v", err)
	}
	if got[1].WriteSeq != 3 || got[2].WriteSeq != 4 {
		t.Fatalf("write seq = %d,%d want 3,4", got[1].WriteSeq, got[2].WriteSeq)
	}
}

func TestEncodeColumnarDenseWideFields(t *testing.T) {
	records := make([]model.ResolvedPoint, 0, 4)
	for index := 0; index < 4; index++ {
		records = append(records, model.ResolvedPoint{
			Measurement: "bench",
			Tags:        map[string]string{"host": "a"},
			SeriesID:    1,
			Timestamp:   int64(index),
			WriteSeq:    uint64(index + 1),
			Fields: []model.ResolvedField{
				{FieldID: 1, FieldName: "f0", Type: model.FieldFloat64, Value: model.Float64Value(float64(index))},
				{FieldID: 2, FieldName: "f1", Type: model.FieldFloat64, Value: model.Float64Value(float64(index) + 0.1)},
				{FieldID: 3, FieldName: "f2", Type: model.FieldFloat64, Value: model.Float64Value(float64(index) + 0.2)},
			},
		})
	}
	payload, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	decoded, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch() error = %v", err)
	}
	if len(decoded) != len(records) {
		t.Fatalf("decoded rows = %d, want %d", len(decoded), len(records))
	}
	for index, point := range decoded {
		if len(point.Fields) != 3 {
			t.Fatalf("row %d fields = %d, want 3", index, len(point.Fields))
		}
		if point.Fields[0].Value.Float64 != float64(index) {
			t.Fatalf("row %d f0 = %v", index, point.Fields[0].Value.Float64)
		}
	}
}
