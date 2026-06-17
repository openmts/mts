package sstable

import (
	"encoding/binary"
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
		payload, err := marshalValueBlockWithTimestamps(nil, column, sampleTimestamps(column.Samples))
		if err != nil {
			t.Fatalf("marshalValueBlockWithTimestamps(%v) error = %v", column.FieldType, err)
		}
		got, err := unmarshalValueBlockWithTimestamps(payload, sampleTimestamps(column.Samples), Query{Start: 0, End: 100})
		if err != nil {
			t.Fatalf("unmarshalValueBlockWithTimestamps(%v) error = %v", column.FieldType, err)
		}
		if !reflect.DeepEqual(got.Samples, column.Samples) {
			t.Fatalf("samples = %#v, want %#v", got.Samples, column.Samples)
		}
	}
}

func TestValuePageRejectsTruncatedTypedValues(t *testing.T) {
	columns := []model.ColumnData{
		columnFor(model.FieldFloat64, []model.FieldValue{model.Float64Value(1)}),
		columnFor(model.FieldInt64, []model.FieldValue{model.Int64Value(1)}),
		columnFor(model.FieldBool, []model.FieldValue{model.BoolValue(true)}),
		columnFor(model.FieldString, []model.FieldValue{model.StringValue("abc")}),
	}
	for index, column := range columns {
		t.Run(string(rune('0'+index)), func(t *testing.T) {
			rowTimestamps := sampleTimestamps(column.Samples)
			payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
			if err != nil {
				t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
			}
			if _, err := unmarshalValueBlockWithTimestamps(payload[:len(payload)-1], rowTimestamps, Query{Start: 0, End: 100}); err == nil {
				t.Fatal("unmarshalValueBlockWithTimestamps(truncated) error = nil, want error")
			}
		})
	}
}

func TestValuePageAlignedIsCompactAndRoundTrips(t *testing.T) {
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
	payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	plainPayload := appendSampleTimes(nil, column.Samples)
	plainPayload = appendSampleWriteSeqs(plainPayload, column.Samples)
	plainPayload, err = appendSampleValues(plainPayload, column)
	if err != nil {
		t.Fatalf("appendSampleValues() error = %v", err)
	}
	plainEstimate := len(plainPayload)
	if len(payload) >= plainEstimate {
		t.Fatalf("page payload size = %d, want smaller than plain timestamp estimate %d", len(payload), plainEstimate)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got.Samples, column.Samples) {
		t.Fatalf("samples = %#v, want %#v", got.Samples, column.Samples)
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
		t.Fatal("unmarshalValueBlockWithTimestamps(page index) error = nil, want error")
	}
}

func TestValuePageIndexRejectsMalformedPayload(t *testing.T) {
	if _, err := unmarshalValuePageIndex([]byte{valueEncodingPagePlain}); err == nil {
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

func TestValuePageIndexHeaderOverflowAndHook(t *testing.T) {
	payload := []byte{valueEncodingPageIndex}
	payload = binary.AppendUvarint(payload, uint64(^uint32(0))+1)
	payload = append(payload, byte(model.FieldFloat64), 0, 0)
	if _, err := unmarshalValuePageIndex(payload); err == nil {
		t.Fatal("unmarshalValuePageIndex(field overflow) error = nil, want error")
	}

	calls := 0
	valuePageRefReadHook = func() { calls++ }
	defer func() {
		valuePageRefReadHook = nil
	}()
	encoded, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     1,
		Pages:     []valuePageRef{{MinTime: 1, MaxTime: 2, Ref: blockRef{Offset: 3, Size: 4}}},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	if _, err := unmarshalValuePageIndex(encoded); err != nil {
		t.Fatalf("unmarshalValuePageIndex() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("valuePageRefReadHook calls = %d, want 1", calls)
	}
}

func TestValuePageIndexedSparseRoundTripAndFilters(t *testing.T) {
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

func TestReadIndexedTimeRefs(t *testing.T) {
	rowTimestamps := []int64{10, 20, 30, 40}
	payload := appendOrdinals(nil, []int{1, 3})
	got, err := readIndexedTimeRefs(newBlockReader(payload), 2, rowTimestamps)
	if err != nil {
		t.Fatalf("readIndexedTimeRefs() error = %v", err)
	}
	if !reflect.DeepEqual(got, []int64{20, 40}) {
		t.Fatalf("timestamps = %#v, want [20 40]", got)
	}
	if _, err := readIndexedTimeRefs(newBlockReader(appendOrdinals(nil, []int{4})), 1, rowTimestamps); err == nil {
		t.Fatal("readIndexedTimeRefs(out of range) error = nil, want error")
	}
}

func TestValuePageIndexedStreamingAllocations(t *testing.T) {
	rowTimestamps := make([]int64, 128)
	samples := make([]model.VersionedSample, 0, len(rowTimestamps)/2)
	for index := range rowTimestamps {
		rowTimestamps[index] = int64(index)
		if index%2 == 0 {
			samples = append(samples, model.VersionedSample{
				Timestamp: int64(index),
				WriteSeq:  uint64(index + 1),
				Value:     model.Float64Value(float64(index)),
			})
		}
	}
	payload, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldID:   3,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 64, End: 64})
		if err != nil {
			t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
		}
		if len(got.Samples) != 1 || got.Samples[0].Timestamp != 64 {
			t.Fatalf("samples = %#v, want timestamp 64", got.Samples)
		}
	})
	if allocs > 2 {
		t.Fatalf("indexed page allocs/run = %.2f, want <= 2", allocs)
	}
}

func TestValuePageFloatAndIntSamplesFilterInDirectReader(t *testing.T) {
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

func TestUnmarshalValueBlockWithTimestampsStreamsAlignedFloatSamples(t *testing.T) {
	rowTimestamps := make([]int64, 128)
	samples := make([]model.VersionedSample, 128)
	for index := range samples {
		rowTimestamps[index] = int64(index)
		samples[index] = model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(index)),
		}
	}
	payload, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldID:   2,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	var got valueBlock
	allocs := testing.AllocsPerRun(100, func() {
		got, err = unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 0, End: 127})
		if err != nil {
			t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
		}
	})
	if allocs > 1 {
		t.Fatalf("aligned float decode allocs/run = %.2f, want <= 1", allocs)
	}
	if !reflect.DeepEqual(got.Samples, samples) {
		t.Fatalf("samples = %#v, want %#v", got.Samples, samples)
	}
}

func TestReadBoolSamplesDirect(t *testing.T) {
	rowTimestamps := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	samples := make([]model.VersionedSample, 0, len(rowTimestamps))
	for index, timestamp := range rowTimestamps {
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  uint64(index + 1),
			Value:     model.BoolValue(index%2 == 0),
		})
	}
	payload, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldID:   3,
		FieldType: model.FieldBool,
		Samples:   samples,
	}, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 2, End: 7})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	want := samples[1:7]
	if !reflect.DeepEqual(got.Samples, want) {
		t.Fatalf("bool samples = %#v, want %#v", got.Samples, want)
	}
}

func TestValuePageIndexedBoolSamples(t *testing.T) {
	rowTimestamps := []int64{1, 2, 3, 4, 5}
	column := model.ColumnData{
		FieldID:   4,
		FieldType: model.FieldBool,
		Samples: []model.VersionedSample{
			{Timestamp: 2, WriteSeq: 2, Value: model.BoolValue(true)},
			{Timestamp: 5, WriteSeq: 5, Value: model.BoolValue(false)},
		},
	}
	payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 1, End: 5})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got.Samples, column.Samples) {
		t.Fatalf("indexed bool samples = %#v, want %#v", got.Samples, column.Samples)
	}
}

func TestValuePageIndexedSamplesAllScalarTypes(t *testing.T) {
	rowTimestamps := []int64{1, 2, 3, 4, 5}
	columns := []model.ColumnData{
		{
			FieldID:   5,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 2, WriteSeq: 2, Value: model.Float64Value(2.5)},
				{Timestamp: 5, WriteSeq: 5, Value: model.Float64Value(5.5)},
			},
		},
		{
			FieldID:   6,
			FieldType: model.FieldInt64,
			Samples: []model.VersionedSample{
				{Timestamp: 2, WriteSeq: 2, Value: model.Int64Value(2)},
				{Timestamp: 5, WriteSeq: 5, Value: model.Int64Value(5)},
			},
		},
		{
			FieldID:   7,
			FieldType: model.FieldString,
			Samples: []model.VersionedSample{
				{Timestamp: 2, WriteSeq: 2, Value: model.StringValue("two")},
				{Timestamp: 5, WriteSeq: 5, Value: model.StringValue("five")},
			},
		},
	}
	for _, column := range columns {
		payload, err := marshalValueBlockWithTimestamps(nil, column, rowTimestamps)
		if err != nil {
			t.Fatalf("marshalValueBlockWithTimestamps(%d) error = %v", column.FieldType, err)
		}
		got, err := unmarshalValueBlockWithTimestamps(payload, rowTimestamps, Query{Start: 1, End: 5})
		if err != nil {
			t.Fatalf("unmarshalValueBlockWithTimestamps(%d) error = %v", column.FieldType, err)
		}
		if !reflect.DeepEqual(got.Samples, column.Samples) {
			t.Fatalf("indexed samples = %#v, want %#v", got.Samples, column.Samples)
		}
	}
}

func TestValuePageAlignedRejectsTruncatedValues(t *testing.T) {
	rowTimestamps := []int64{1}
	cases := []struct {
		name      string
		fieldType model.FieldType
		value     model.FieldValue
	}{
		{name: "float", fieldType: model.FieldFloat64, value: model.Float64Value(1)},
		{name: "int", fieldType: model.FieldInt64, value: model.Int64Value(1)},
		{name: "bool", fieldType: model.FieldBool, value: model.BoolValue(true)},
		{name: "string", fieldType: model.FieldString, value: model.StringValue("ok")},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
				FieldID:   9,
				FieldType: tt.fieldType,
				Samples: []model.VersionedSample{{
					Timestamp: 1,
					WriteSeq:  1,
					Value:     tt.value,
				}},
			}, rowTimestamps)
			if err != nil {
				t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
			}
			truncated := payload[:len(payload)-1]
			if _, err := unmarshalValueBlockWithTimestamps(truncated, rowTimestamps, Query{Start: 0, End: 2}); err == nil {
				t.Fatal("unmarshalValueBlockWithTimestamps(truncated) error = nil, want error")
			}
		})
	}
}

func TestUnmarshalValueBlockWithTimestampsRejectsEmptyPayload(t *testing.T) {
	if _, err := unmarshalValueBlockWithTimestamps(nil, nil, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(empty) error = nil, want error")
	}
}

func TestEmptyValueBlockAndTimeRefs(t *testing.T) {
	payload, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldID:   11,
		FieldType: model.FieldString,
	}, nil)
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps(empty) error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, nil, Query{Start: 0, End: 1})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps(empty) error = %v", err)
	}
	if got.FieldID != 11 || got.FieldType != model.FieldString || len(got.Samples) != 0 {
		t.Fatalf("empty value block = %#v, want header only", got)
	}
	mode, ordinals, err := encodeTimeRefs(nil, nil)
	if err != nil {
		t.Fatalf("encodeTimeRefs(empty) error = %v", err)
	}
	if mode != timeRefModeAligned || ordinals != nil {
		t.Fatalf("encodeTimeRefs(empty) mode=%d ordinals=%v, want aligned nil", mode, ordinals)
	}
	if got := marshalTimeBlock(nil, nil); len(got) != 2 {
		t.Fatalf("marshalTimeBlock(empty) len = %d, want 2", len(got))
	}
}

func TestValuePageRejectsInvalidTimeReferences(t *testing.T) {
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
		valueEncodingPagePlain,
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

func TestValuePageRejectsMalformedModesAndTypes(t *testing.T) {
	alignedMismatch := []byte{
		valueEncodingPagePlain,
		2,
		byte(model.FieldFloat64),
		2,
		timeRefModeAligned,
	}
	if _, err := unmarshalValueBlockWithTimestamps(alignedMismatch, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(aligned mismatch) error = nil, want error")
	}

	unknownMode := []byte{
		valueEncodingPagePlain,
		2,
		byte(model.FieldFloat64),
		1,
		99,
	}
	if _, err := unmarshalValueBlockWithTimestamps(unknownMode, []int64{10}, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unknown mode) error = nil, want error")
	}

	zeroDelta := []byte{
		valueEncodingPagePlain,
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
		valueEncodingPagePlain,
		2,
		99,
		0,
		timeRefModeAligned,
	}
	if _, err := unmarshalValueBlockWithTimestamps(unsupportedType, nil, Query{Start: 0, End: 100}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unsupported type) error = nil, want error")
	}
}

func TestValuePageBoolAndStringSamples(t *testing.T) {
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

func TestLegacyValueReadersAndFilter(t *testing.T) {
	timestamps := []int64{1, 2, 3}
	writeSeqs := []uint64{10, 20, 30}
	query := Query{Start: 2, End: 2}
	for _, item := range []struct {
		name      string
		fieldType model.FieldType
		payload   []byte
		want      model.FieldValue
	}{
		{name: "float", fieldType: model.FieldFloat64, payload: appendFloatValues(nil, []model.VersionedSample{{Value: model.Float64Value(1)}, {Value: model.Float64Value(2)}, {Value: model.Float64Value(3)}}), want: model.Float64Value(2)},
		{name: "int", fieldType: model.FieldInt64, payload: appendIntValues(nil, []model.VersionedSample{{Value: model.Int64Value(1)}, {Value: model.Int64Value(2)}, {Value: model.Int64Value(3)}}), want: model.Int64Value(2)},
		{name: "bool", fieldType: model.FieldBool, payload: mustAppendBoolValues(t, []model.VersionedSample{{Value: model.BoolValue(false)}, {Value: model.BoolValue(true)}, {Value: model.BoolValue(false)}}), want: model.BoolValue(true)},
		{name: "string", fieldType: model.FieldString, payload: appendStringValues(nil, []model.VersionedSample{{Value: model.StringValue("a")}, {Value: model.StringValue("b")}, {Value: model.StringValue("c")}}), want: model.StringValue("b")},
	} {
		t.Run(item.name, func(t *testing.T) {
			values, err := readValues(newBlockReader(item.payload), item.fieldType, 3)
			if err != nil {
				t.Fatalf("readValues() error = %v", err)
			}
			if !reflect.DeepEqual(values[1], item.want) {
				t.Fatalf("readValues()[1] = %#v, want %#v", values[1], item.want)
			}
			samples, err := readSamples(newBlockReader(item.payload), item.fieldType, timestamps, writeSeqs, query)
			if err != nil {
				t.Fatalf("readSamples() error = %v", err)
			}
			if len(samples) != 1 || !reflect.DeepEqual(samples[0].Value, item.want) || samples[0].WriteSeq != 20 {
				t.Fatalf("readSamples() = %#v, want timestamp 2 value %#v", samples, item.want)
			}
		})
	}
	if _, err := readValues(newBlockReader(nil), model.FieldType(99), 1); err == nil {
		t.Fatal("readValues(unsupported) error = nil, want error")
	}
	if _, err := readSamples(newBlockReader(nil), model.FieldType(99), timestamps, writeSeqs, query); err == nil {
		t.Fatal("readSamples(unsupported) error = nil, want error")
	}

	block := valueBlock{Samples: []model.VersionedSample{{Timestamp: 1}, {Timestamp: 2}, {Timestamp: 3}}}
	got := filterValueBlock(block, Query{Start: 2, End: 3})
	if len(got.Samples) != 2 {
		t.Fatalf("filterValueBlock() len = %d, want 2", len(got.Samples))
	}
	got = filterValueBlock(block, Query{Start: 4, End: 3})
	if len(got.Samples) != 0 {
		t.Fatalf("filterValueBlock(empty range) len = %d, want 0", len(got.Samples))
	}
}

func mustAppendBoolValues(t *testing.T, samples []model.VersionedSample) []byte {
	t.Helper()
	payload, err := appendBoolValues(nil, samples)
	if err != nil {
		t.Fatalf("appendBoolValues() error = %v", err)
	}
	return payload
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

func sampleTimestamps(samples []model.VersionedSample) []int64 {
	timestamps := make([]int64, len(samples))
	for index, sample := range samples {
		timestamps[index] = sample.Timestamp
	}
	return timestamps
}
