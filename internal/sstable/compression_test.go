package sstable

import (
	"encoding/binary"
	"reflect"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestTimestampDeltaOfDeltaIsSmallerForRegularSeries(t *testing.T) {
	timestamps := make([]int64, 128)
	for index := range timestamps {
		timestamps[index] = int64(index * 1_000)
	}
	plain := appendPlainTimestamps(nil, timestamps)
	codecID, payload, err := encodeTimestamps(timestamps, "delta-of-delta")
	if err != nil {
		t.Fatalf("encodeTimestamps() error = %v", err)
	}
	if codecID != compressionDeltaOfDelta {
		t.Fatalf("timestamp codec = %d, want delta-of-delta", codecID)
	}
	if len(payload) >= len(plain) {
		t.Fatalf("delta-of-delta size = %d, want smaller than plain %d", len(payload), len(plain))
	}
	got, err := decodeDeltaOfDeltaTimestamps(payload, len(timestamps))
	if err != nil {
		t.Fatalf("decodeDeltaOfDeltaTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("timestamps = %#v, want %#v", got, timestamps)
	}
}

func TestFloatXOREncodingFallsBackOnlyWhenUseful(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 128, func(index int) model.FieldValue {
		return model.Float64Value(42)
	})
	plain := appendFloatValues(nil, samples)
	codecID, payload, err := encodeFloatValues(samples, "xor")
	if err != nil {
		t.Fatalf("encodeFloatValues() error = %v", err)
	}
	if codecID != compressionXOR {
		t.Fatalf("float codec = %d, want xor", codecID)
	}
	if len(payload) >= len(plain) {
		t.Fatalf("xor size = %d, want smaller than plain %d", len(payload), len(plain))
	}
}

func TestIntDeltaEncodingIsSmallerForIncreasingLargeValues(t *testing.T) {
	samples := makeSamples(model.FieldInt64, 128, func(index int) model.FieldValue {
		return model.Int64Value(1_000_000_000_000 + int64(index))
	})
	plain := appendIntValues(nil, samples)
	codecID, payload, err := encodeIntValues(samples, "delta")
	if err != nil {
		t.Fatalf("encodeIntValues() error = %v", err)
	}
	if codecID != compressionDelta {
		t.Fatalf("int codec = %d, want delta", codecID)
	}
	if len(payload) >= len(plain) {
		t.Fatalf("delta size = %d, want smaller than plain %d", len(payload), len(plain))
	}
}

func TestStringDictionaryEncodingIsSmallerForRepeatedValues(t *testing.T) {
	values := []string{"alpha", "beta", "alpha", "alpha", "beta", "gamma"}
	samples := makeSamples(model.FieldString, 120, func(index int) model.FieldValue {
		return model.StringValue(values[index%len(values)])
	})
	plain := appendStringValues(nil, samples)
	codecID, payload, err := encodeStringValues(samples, "dictionary")
	if err != nil {
		t.Fatalf("encodeStringValues() error = %v", err)
	}
	if codecID != compressionDictionary {
		t.Fatalf("string codec = %d, want dictionary", codecID)
	}
	if len(payload) >= len(plain) {
		t.Fatalf("dictionary size = %d, want smaller than plain %d", len(payload), len(plain))
	}
}

func TestCompressedValueBlockRoundTripsAndFilters(t *testing.T) {
	column := model.ColumnData{
		FieldID:   9,
		FieldType: model.FieldInt64,
		Samples: makeSamples(model.FieldInt64, 32, func(index int) model.FieldValue {
			return model.Int64Value(10_000_000_000 + int64(index))
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{Enabled: true, MinPageValues: 1})
	if err != nil {
		t.Fatalf("marshalCompressedValueBlock() error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, nil, Query{Start: 10, End: 12})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	if len(got.Samples) != 3 {
		t.Fatalf("filtered sample count = %d, want 3", len(got.Samples))
	}
	for index, sample := range got.Samples {
		wantTime := int64(index + 10)
		if sample.Timestamp != wantTime {
			t.Fatalf("sample[%d] timestamp = %d, want %d", index, sample.Timestamp, wantTime)
		}
	}
}

func TestCompressedStringDictionaryRoundTrips(t *testing.T) {
	values := []string{"alpha", "beta", "alpha", "gamma"}
	column := model.ColumnData{
		FieldID:   10,
		FieldType: model.FieldString,
		Samples: makeSamples(model.FieldString, 64, func(index int) model.FieldValue {
			return model.StringValue(values[index%len(values)])
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{Enabled: true, MinPageValues: 1})
	if err != nil {
		t.Fatalf("marshalCompressedValueBlock() error = %v", err)
	}
	got, err := unmarshalValueBlockWithTimestamps(payload, nil, Query{Start: 0, End: 63})
	if err != nil {
		t.Fatalf("unmarshalValueBlockWithTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got.Samples, column.Samples) {
		t.Fatalf("samples = %#v, want %#v", got.Samples, column.Samples)
	}
}

func TestCompressedPlainTimestampPolicyRoundTrips(t *testing.T) {
	timestamps := []int64{1, 3, 8}
	codecID, payload, err := encodeTimestamps(timestamps, "plain")
	if err != nil {
		t.Fatalf("encodeTimestamps(plain) error = %v", err)
	}
	if codecID != compressionPlain {
		t.Fatalf("timestamp codec = %d, want plain", codecID)
	}
	got, err := decodePlainTimestamps(payload, len(timestamps))
	if err != nil {
		t.Fatalf("decodePlainTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("timestamps = %#v, want %#v", got, timestamps)
	}
}

func TestCompressedValueBlockRejectsMalformedPayloads(t *testing.T) {
	if _, err := unmarshalCompressedValueBlock([]byte{valueEncodingV5}, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalCompressedValueBlock(short) error = nil, want error")
	}
	payload := []byte{
		valueEncodingV5,
		1,
		byte(model.FieldFloat64),
		1,
		99,
		0,
	}
	if _, err := unmarshalCompressedValueBlock(payload, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalCompressedValueBlock(bad timestamp codec) error = nil, want error")
	}
}

func TestCompressedBoolPlainAndUnknownValueCodec(t *testing.T) {
	column := model.ColumnData{
		FieldID:   11,
		FieldType: model.FieldBool,
		Samples: makeSamples(model.FieldBool, 16, func(index int) model.FieldValue {
			return model.BoolValue(index%2 == 0)
		}),
	}
	codecID, payload, err := encodeTypedValues(column, model.CompressionOptions{})
	if err != nil {
		t.Fatalf("encodeTypedValues(bool) error = %v", err)
	}
	if codecID != compressionPlain || len(payload) == 0 {
		t.Fatalf("bool codec=%d payload len=%d, want plain payload", codecID, len(payload))
	}
	reader := newBlockReader(nil)
	if _, err := readCompressedValues(reader, model.FieldBool, compressionXOR, 1); err == nil {
		t.Fatal("readCompressedValues(bool xor) error = nil, want error")
	}
}

func TestCompressedHeadersAndOptionsBoundaries(t *testing.T) {
	if compressionEnabled(model.CompressionOptions{Enabled: false}, 100) {
		t.Fatal("compressionEnabled(disabled) = true, want false")
	}
	if compressionEnabled(model.CompressionOptions{Enabled: true, MinPageValues: 10}, 9) {
		t.Fatal("compressionEnabled(below min) = true, want false")
	}
	if !compressionEnabled(model.CompressionOptions{Enabled: true, MinPageValues: 10}, 10) {
		t.Fatal("compressionEnabled(at min) = false, want true")
	}
	if _, err := readValueHeaderV5(newBlockReader([]byte{valueEncodingV3})); err == nil {
		t.Fatal("readValueHeaderV5(wrong encoding) error = nil, want error")
	}
	payload := []byte{valueEncodingV5}
	payload = binary.AppendUvarint(payload, uint64(^uint32(0))+1)
	if _, err := readValueHeaderV5(newBlockReader(payload)); err == nil {
		t.Fatal("readValueHeaderV5(field overflow) error = nil, want error")
	}
}

func TestDeltaOfDeltaRejectsInvalidRun(t *testing.T) {
	payload := appendPlainTimestamps(nil, []int64{1, 2})
	payload = append(payload, 0, 0)
	if _, err := decodeDeltaOfDeltaTimestamps(payload, 3); err == nil {
		t.Fatal("decodeDeltaOfDeltaTimestamps(invalid run) error = nil, want error")
	}
}

func TestCompressedCodecReadersRejectInvalidPayloads(t *testing.T) {
	reader := newBlockReader([]byte{compressionPlain, 2, 1})
	if _, err := readCodecTimestamps(reader, 1); err == nil {
		t.Fatal("readCodecTimestamps(truncated) error = nil, want error")
	}
	reader = newBlockReader([]byte{99, 0})
	if _, err := readCodecWriteSeqs(reader, 1); err == nil {
		t.Fatal("readCodecWriteSeqs(unknown) error = nil, want error")
	}
	reader = newBlockReader([]byte{compressionDictionary, 3, 1, 1, 1})
	if _, err := readCodecValues(reader, model.FieldString, 1); err == nil {
		t.Fatal("readCodecValues(bad dictionary) error = nil, want error")
	}
}

func TestCompressionFallbackPoliciesAndEmptyReaders(t *testing.T) {
	floatSamples := makeSamples(model.FieldFloat64, 4, func(index int) model.FieldValue {
		return model.Float64Value(float64(index) + 0.123456789)
	})
	codecID, _, err := encodeFloatValues(floatSamples, "plain")
	if err != nil {
		t.Fatalf("encodeFloatValues(plain) error = %v", err)
	}
	if codecID != compressionPlain {
		t.Fatalf("float plain codec = %d, want plain", codecID)
	}
	intSamples := makeSamples(model.FieldInt64, 4, func(index int) model.FieldValue {
		return model.Int64Value(int64(index))
	})
	codecID, _, err = encodeIntValues(intSamples, "plain")
	if err != nil {
		t.Fatalf("encodeIntValues(plain) error = %v", err)
	}
	if codecID != compressionPlain {
		t.Fatalf("int plain codec = %d, want plain", codecID)
	}
	stringSamples := makeSamples(model.FieldString, 32, func(index int) model.FieldValue {
		return model.StringValue(string(rune('a' + index)))
	})
	codecID, _, err = encodeStringValues(stringSamples, "plain")
	if err != nil {
		t.Fatalf("encodeStringValues(plain) error = %v", err)
	}
	if codecID != compressionPlain {
		t.Fatalf("string plain codec = %d, want plain", codecID)
	}
	if compressionPolicy("unknown", "xor") != "xor" {
		t.Fatal("compressionPolicy(unknown) did not return default")
	}
	if values, err := readXORFloatValues(newBlockReader(nil), compressionXOR, 0); err != nil || len(values) != 0 {
		t.Fatalf("readXORFloatValues(empty) = %v, %v; want empty nil", values, err)
	}
	if values, err := readDeltaIntValues(newBlockReader(nil), compressionDelta, 0); err != nil || len(values) != 0 {
		t.Fatalf("readDeltaIntValues(empty) = %v, %v; want empty nil", values, err)
	}
}

func TestDeltaOfDeltaEmptyAndSingleTimestamp(t *testing.T) {
	got, err := decodeDeltaOfDeltaTimestamps(nil, 0)
	if err != nil || len(got) != 0 {
		t.Fatalf("decode empty = %v, %v; want empty nil", got, err)
	}
	payload := appendDeltaOfDeltaTimestamps(nil, []int64{42})
	got, err = decodeDeltaOfDeltaTimestamps(payload, 1)
	if err != nil {
		t.Fatalf("decode single error = %v", err)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("single timestamp = %v, want [42]", got)
	}
}

func TestReadDictionaryStringValuesRejectsOrdinalOutOfRange(t *testing.T) {
	payload := []byte{1}
	payload = append(payload, 1, 'a')
	payload = append(payload, 1)
	reader := newBlockReader(payload)
	if _, err := readDictionaryStringValues(reader, compressionDictionary, 1); err == nil {
		t.Fatal("readDictionaryStringValues(out of range) error = nil, want error")
	}
}

func makeSamples(
	fieldType model.FieldType,
	count int,
	value func(index int) model.FieldValue,
) []model.VersionedSample {
	samples := make([]model.VersionedSample, count)
	for index := range count {
		samples[index] = model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     value(index),
		}
	}
	return samples
}
