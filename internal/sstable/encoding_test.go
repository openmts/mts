package sstable

import (
	"reflect"
	"slices"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestBinaryBlocksRoundTripAllTypes(t *testing.T) {
	timestamps := []int64{10, 20, 35}
	timePayload := marshalTimeBlock(nil, timestamps)
	gotTimes, err := unmarshalTimeBlock(timePayload)
	if err != nil {
		t.Fatalf("unmarshalTimeBlock() error = %v", err)
	}
	if !slices.Equal(gotTimes, timestamps) {
		t.Fatalf("timestamps = %v, want %v", gotTimes, timestamps)
	}

	columns := []model.ColumnData{
		columnFor(model.FieldFloat64, []model.FieldValue{model.Float64Value(1), model.Float64Value(2)}),
		columnFor(model.FieldInt64, []model.FieldValue{model.Int64Value(1), model.Int64Value(2)}),
		columnFor(model.FieldBool, []model.FieldValue{model.BoolValue(true), model.BoolValue(false)}),
		columnFor(model.FieldString, []model.FieldValue{model.StringValue("a"), model.StringValue("bb")}),
	}
	for _, column := range columns {
		payload, err := marshalValueBlock(nil, column)
		if err != nil {
			t.Fatalf("marshalValueBlock(%v) error = %v", column.FieldType, err)
		}
		got, err := unmarshalValueBlock(payload)
		if err != nil {
			t.Fatalf("unmarshalValueBlock(%v) error = %v", column.FieldType, err)
		}
		if !reflect.DeepEqual(got.Samples, column.Samples) {
			t.Fatalf("samples = %#v, want %#v", got.Samples, column.Samples)
		}
	}
}

func columnFor(fieldType model.FieldType, values []model.FieldValue) model.ColumnData {
	samples := make([]model.VersionedSample, 0, len(values))
	for index, value := range values {
		samples = append(samples, model.VersionedSample{
			Timestamp: int64(10 + index*10),
			WriteSeq:  uint64(index + 1),
			Value:     value,
		})
	}
	return model.ColumnData{
		SeriesID:  1,
		FieldID:   uint32(fieldType),
		FieldType: fieldType,
		Samples:   samples,
	}
}
