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

func TestValueBlockV3AlignedIsCompactAndRoundTrips(t *testing.T) {
	rowTimestamps := []int64{10, 20, 30, 40}
	column := model.ColumnData{
		SeriesID:  1,
		FieldID:   2,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
			{Timestamp: 30, WriteSeq: 3, Value: model.Float64Value(3)},
			{Timestamp: 40, WriteSeq: 4, Value: model.Float64Value(4)},
		},
	}
	v2, err := marshalValueBlock(nil, column)
	if err != nil {
		t.Fatalf("marshalValueBlock(v2) error = %v", err)
	}
	v3, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	if len(v3) >= len(v2) {
		t.Fatalf("v3 size = %d, want smaller than v2 size %d", len(v3), len(v2))
	}
	got, err := unmarshalValueBlockWithTimestamps(v3, rowTimestamps, Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got.Samples, column.Samples) {
		t.Fatalf("v3 samples = %#v, want %#v", got.Samples, column.Samples)
	}
}

func TestValuePageIndexRoundTrip(t *testing.T) {
	index := valuePageIndex{
		FieldID:   7,
		FieldType: model.FieldFloat64,
		Count:     3,
		Pages: []valuePageRef{
			{MinTime: 10, MaxTime: 20, Ref: blockRef{Offset: 1, Size: 2}},
			{MinTime: 30, MaxTime: 40, Ref: blockRef{Offset: 3, Size: 4}},
		},
	}
	payload, err := marshalValuePageIndex(nil, index)
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	got, err := unmarshalValuePageIndex(payload)
	if err != nil {
		t.Fatalf("unmarshalValuePageIndex() error = %v", err)
	}
	if !reflect.DeepEqual(got, index) {
		t.Fatalf("page index = %#v, want %#v", got, index)
	}
	if _, err := unmarshalValueBlockWithTimestamps(payload, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(v4 index) error = nil, want error")
	}
}

func TestValuePageIndexRejectsMalformedPayload(t *testing.T) {
	if _, err := unmarshalValuePageIndex([]byte{valueEncodingV3}); err == nil {
		t.Fatal("unmarshalValuePageIndex(wrong encoding) error = nil, want error")
	}
	if _, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     1,
		Pages:     []valuePageRef{{Ref: blockRef{Offset: -1}}},
	}); err == nil {
		t.Fatal("marshalValuePageIndex(negative ref) error = nil, want error")
	}
}

func TestValueBlockV3IndexedSparseRoundTripAndFilters(t *testing.T) {
	rowTimestamps := []int64{10, 20, 30, 40, 50}
	column := model.ColumnData{
		SeriesID:  1,
		FieldID:   3,
		FieldType: model.FieldInt64,
		Samples: []model.VersionedSample{
			{Timestamp: 20, WriteSeq: 2, Value: model.Int64Value(20)},
			{Timestamp: 50, WriteSeq: 5, Value: model.Int64Value(50)},
		},
	}
	payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 30, End: 60})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	want := column.Samples[1:]
	if !reflect.DeepEqual(got.Samples, want) {
		t.Fatalf("filtered samples = %#v, want %#v", got.Samples, want)
	}
}

func TestValueBlockV3FloatAndIntSamplesFilterInDirectReader(t *testing.T) {
	rowTimestamps := []int64{10, 20, 30}
	cases := []struct {
		name   string
		column model.ColumnData
		want   []model.VersionedSample
	}{
		{
			name: "float64",
			column: model.ColumnData{
				FieldID:   2,
				FieldType: model.FieldFloat64,
				Samples: []model.VersionedSample{
					{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
					{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
					{Timestamp: 30, WriteSeq: 3, Value: model.Float64Value(3)},
				},
			},
			want: []model.VersionedSample{
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
			},
		},
		{
			name: "int64",
			column: model.ColumnData{
				FieldID:   3,
				FieldType: model.FieldInt64,
				Samples: []model.VersionedSample{
					{Timestamp: 10, WriteSeq: 1, Value: model.Int64Value(1)},
					{Timestamp: 20, WriteSeq: 2, Value: model.Int64Value(2)},
					{Timestamp: 30, WriteSeq: 3, Value: model.Int64Value(3)},
				},
			},
			want: []model.VersionedSample{
				{Timestamp: 20, WriteSeq: 2, Value: model.Int64Value(2)},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := marshalValueBlockWithTimestamps(nil, tt.column, rowTimestamps)
			if err != nil {
				t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
			}
			got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 20, End: 20})
			if err != nil {
				t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
			}
			if !reflect.DeepEqual(got.Samples, tt.want) {
				t.Fatalf("samples = %#v, want %#v", got.Samples, tt.want)
			}
		})
	}
}

func TestUnmarshalValueBlockWithTimestampsRejectsEmptyPayload(t *testing.T) {
	if _, err := unmarshalValueBlockWithTimestamps(nil, nil, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(empty) error = nil, want error")
	}
}

func TestValueBlockV3RejectsInvalidTimeReferences(t *testing.T) {
	column := model.ColumnData{
		FieldID:   2,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: 99, WriteSeq: 1, Value: model.Float64Value(1)},
		},
	}
	if _, err := marshalValueBlockWithTimestamps(nil, column, []int64{10}); err == nil {
		t.Fatal("marshalValueBlockWithTimestamps(missing timestamp) error = nil, want error")
	}
	payload := []byte{
		valueEncodingV3,
		2,
		byte(model.FieldFloat64),
		1,
		timeRefModeIndexed,
		9,
		1,
	}
	payload = append(payload, 0, 0, 0, 0, 0, 0, 0xf0, 0x3f)
	if _, err := unmarshalValueBlockWithTimestamps(payload, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(out of range ordinal) error = nil, want error")
	}
	if _, err := unmarshalValueBlockWithTimestamps(payload, nil, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(missing row timestamps) error = nil, want error")
	}
}

func TestValueBlockV3RejectsMalformedModesAndTypes(t *testing.T) {
	alignedMismatch := []byte{
		valueEncodingV3,
		2,
		byte(model.FieldFloat64),
		2,
		timeRefModeAligned,
	}
	if _, err := unmarshalValueBlockWithTimestamps(alignedMismatch, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(aligned mismatch) error = nil, want error")
	}

	unknownMode := []byte{
		valueEncodingV3,
		2,
		byte(model.FieldFloat64),
		1,
		99,
	}
	if _, err := unmarshalValueBlockWithTimestamps(unknownMode, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unknown mode) error = nil, want error")
	}

	zeroDelta := []byte{
		valueEncodingV3,
		2,
		byte(model.FieldFloat64),
		2,
		timeRefModeIndexed,
		0,
		0,
	}
	if _, err := unmarshalValueBlockWithTimestamps(zeroDelta, []int64{10, 20}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(zero ordinal delta) error = nil, want error")
	}

	unsupportedType := []byte{
		valueEncodingV3,
		2,
		99,
		0,
		timeRefModeAligned,
	}
	if _, err := unmarshalValueBlockWithTimestamps(unsupportedType, nil, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unsupported type) error = nil, want error")
	}
}

func TestValueBlockV3BoolAndStringSamples(t *testing.T) {
	rowTimestamps := []int64{10, 20}
	cases := []model.ColumnData{
		{
			FieldID:   4,
			FieldType: model.FieldBool,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.BoolValue(true)},
				{Timestamp: 20, WriteSeq: 2, Value: model.BoolValue(false)},
			},
		},
		{
			FieldID:   5,
			FieldType: model.FieldString,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.StringValue("a")},
				{Timestamp: 20, WriteSeq: 2, Value: model.StringValue("bb")},
			},
		},
	}
	for _, column := range cases {
		payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
		if err != nil {
			t.Fatalf("marshalValueBlockWithTimestamps(%d) error = %v", column.FieldType, err)
		}
		got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 15, End: 25})
		if err != nil {
			t.Fatalf("unmarshalValueBlockWithTimestamps(%d) error = %v", column.FieldType, err)
		}
		want := column.Samples[1:]
		if !reflect.DeepEqual(got.Samples, want) {
			t.Fatalf("samples for type %d = %#v, want %#v", column.FieldType, got.Samples, want)
		}
	}
}

func TestValueBlockV2CompatibilityThroughTimestampDecoder(t *testing.T) {
	column := columnFor(model.FieldString, []model.FieldValue{model.StringValue("old")})
	payload, err := marshalValueBlock(nil, column)
	if err != nil {
		t.Fatalf("marshalValueBlock(v2) error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, nil, Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps(v2) error = %v", err)
	}
	if !reflect.DeepEqual(got.Samples, column.Samples) {
		t.Fatalf("v2 samples = %#v, want %#v", got.Samples, column.Samples)
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
