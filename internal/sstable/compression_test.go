package sstable

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"codeberg.org/mts/mts/internal/model"
)

func TestPayloadCompressionAlgorithmsRoundTrip(t *testing.T) {
	source := bytes.Repeat([]byte("mts-payload-compression-"), 64)
	algorithms := []string{"", "none", "snappy", "lz4", "zstd"}
	for _, algorithm := range algorithms {
		t.Run(algorithm, func(t *testing.T) {
			payload, err := appendCodecPayloadWithCompression(nil, compressionDelta, source, algorithm)
			if err != nil {
				t.Fatalf("appendCodecPayloadWithCompression(%q) error = %v", algorithm, err)
			}
			codecID, got, err := readCodecPayload(newBlockReader(payload), "test payload")
			if err != nil {
				t.Fatalf("readCodecPayload(%q) error = %v", algorithm, err)
			}
			if codecID != compressionDelta {
				t.Fatalf("codecID = %d, want %d", codecID, compressionDelta)
			}
			if !bytes.Equal(got, source) {
				t.Fatalf("payload mismatch for %q", algorithm)
			}
		})
	}
}

func TestPayloadCompressionRejectsUnknownAlgorithm(t *testing.T) {
	_, err := appendCodecPayloadWithCompression(nil, compressionPlain, []byte("payload"), "gzip")
	if err == nil {
		t.Fatal("appendCodecPayloadWithCompression(gzip) error = nil, want error")
	}
}

func TestCompressedPayloadRejectsCorruptSize(t *testing.T) {
	payload := []byte{compressionPlain, payloadCompressionNone}
	payload = binary.AppendUvarint(payload, 4)
	payload = binary.AppendUvarint(payload, 3)
	payload = append(payload, "abc"...)
	if _, _, err := readCodecPayload(newBlockReader(payload), "corrupt payload"); err == nil {
		t.Fatal("readCodecPayload(corrupt size) error = nil, want error")
	}
}

func TestCompressedPayloadRejectsUnknownAlgorithmID(t *testing.T) {
	payload := []byte{compressionPlain, 99}
	payload = binary.AppendUvarint(payload, 1)
	payload = binary.AppendUvarint(payload, 1)
	payload = append(payload, 'x')
	if _, _, err := readCodecPayload(newBlockReader(payload), "unknown payload"); err == nil {
		t.Fatal("readCodecPayload(unknown algorithm) error = nil, want error")
	}
}

func TestPayloadCompressionRejectsMalformedHeaders(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{name: "missing algorithm", payload: []byte{compressionPlain}},
		{name: "bad raw size", payload: []byte{compressionPlain, payloadCompressionNone, 0x80}},
		{name: "bad stored size", payload: []byte{compressionPlain, payloadCompressionNone, 1, 0x80}},
		{name: "truncated payload", payload: []byte{compressionPlain, payloadCompressionNone, 1, 2, 'a'}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := readCodecPayload(newBlockReader(tt.payload), tt.name); err == nil {
				t.Fatal("readCodecPayload() error = nil, want error")
			}
		})
	}
}

func TestPayloadCompressionInternalErrorBranches(t *testing.T) {
	if _, err := compressPayload(99, []byte("payload")); err == nil {
		t.Fatal("compressPayload(unknown) error = nil, want error")
	}
	if _, err := decompressPayload(payloadCompressionSnappy, []byte("bad"), 10); err == nil {
		t.Fatal("decompressPayload(bad snappy) error = nil, want error")
	}
	if _, err := decompressPayload(payloadCompressionLZ4, []byte{0x10, 'a'}, 2); err == nil {
		t.Fatal("decompressPayload(bad lz4) error = nil, want error")
	}
}

func TestLZ4BlockRejectsMalformedPayloads(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		rawSize int
	}{
		{name: "literal too long", payload: []byte{0x20, 'a'}, rawSize: 2},
		{name: "truncated offset", payload: []byte{0x00, 0x01}, rawSize: 4},
		{name: "zero offset", payload: []byte{0x00, 0x00, 0x00}, rawSize: 4},
		{name: "offset beyond output", payload: []byte{0x00, 0x01, 0x00}, rawSize: 4},
		{name: "match too long", payload: []byte{0x1f, 'a', 0x01, 0x00, 0x00}, rawSize: 2},
		{name: "truncated extended length", payload: []byte{0xf0, 0xff}, rawSize: 20},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeLZ4Block(nil, tt.payload, tt.rawSize); err == nil {
				t.Fatal("decodeLZ4Block() error = nil, want error")
			}
		})
	}
}

func TestAppendSampleWriteSeqsPayloadRoundTrip(t *testing.T) {
	samples := makeSamples(model.FieldInt64, 3, func(index int) model.FieldValue {
		return model.Int64Value(int64(index))
	})
	codecID, payload, err := readCodecPayload(newBlockReader(appendSampleWriteSeqsPayload(nil, samples)), "write seqs")
	if err != nil {
		t.Fatalf("readCodecPayload(write seqs) error = %v", err)
	}
	got, err := decodeCodecWriteSeqs(codecID, payload, len(samples))
	if err != nil {
		t.Fatalf("decodeCodecWriteSeqs() error = %v", err)
	}
	if !reflect.DeepEqual(got, []uint64{1, 2, 3}) {
		t.Fatalf("write seqs = %#v, want [1 2 3]", got)
	}
}

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

func TestPlainValueCodecRoundTripsAllTypes(t *testing.T) {
	cases := []model.ColumnData{
		{
			FieldType: model.FieldFloat64,
			Samples: makeSamples(model.FieldFloat64, 3, func(index int) model.FieldValue {
				return model.Float64Value(float64(index) + 0.5)
			}),
		},
		{
			FieldType: model.FieldInt64,
			Samples: makeSamples(model.FieldInt64, 3, func(index int) model.FieldValue {
				return model.Int64Value(int64(index - 1))
			}),
		},
		{
			FieldType: model.FieldBool,
			Samples: makeSamples(model.FieldBool, 3, func(index int) model.FieldValue {
				return model.BoolValue(index%2 == 0)
			}),
		},
		{
			FieldType: model.FieldString,
			Samples: makeSamples(model.FieldString, 3, func(index int) model.FieldValue {
				return model.StringValue([]string{"a", "bb", "ccc"}[index])
			}),
		},
	}
	for _, column := range cases {
		codecID, payload, err := encodeTypedValues(column, model.CompressionOptions{
			Float:  "plain",
			Int:    "plain",
			String: "plain",
		})
		if err != nil {
			t.Fatalf("encodeTypedValues(%d) error = %v", column.FieldType, err)
		}
		if codecID != compressionPlain {
			t.Fatalf("codec(%d) = %d, want plain", column.FieldType, codecID)
		}
		got, err := readCompressedValues(newBlockReader(payload), column.FieldType, codecID, len(column.Samples))
		if err != nil {
			t.Fatalf("readCompressedValues(%d) error = %v", column.FieldType, err)
		}
		want := sampleValues(column.Samples)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("values(%d) = %#v, want %#v", column.FieldType, got, want)
		}
	}
}

func TestCompressedPlainValueCodecStreamsSamples(t *testing.T) {
	column := model.ColumnData{
		FieldID:   12,
		FieldType: model.FieldFloat64,
		Samples: makeSamples(model.FieldFloat64, 64, func(index int) model.FieldValue {
			return model.Float64Value(float64(index) + 0.5)
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "plain",
		Float:         "plain",
	})
	if err != nil {
		t.Fatalf("marshalCompressedValueBlock() error = %v", err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		got, err := unmarshalCompressedValueBlock(payload, Query{Start: 10, End: 10})
		if err != nil {
			t.Fatalf("unmarshalCompressedValueBlock() error = %v", err)
		}
		if len(got.Samples) != 1 || got.Samples[0].Timestamp != 10 {
			t.Fatalf("samples = %#v, want timestamp 10", got.Samples)
		}
	})
	if allocs > 3 {
		t.Fatalf("compressed plain value allocs/run = %.2f, want <= 3", allocs)
	}
}

func TestCompressedPlainValueCodecStreamsAllScalarTypes(t *testing.T) {
	cases := []struct {
		name      string
		fieldType model.FieldType
		value     func(int) model.FieldValue
		options   model.CompressionOptions
		check     func(t *testing.T, sample model.VersionedSample)
	}{
		{
			name:      "int",
			fieldType: model.FieldInt64,
			value: func(index int) model.FieldValue {
				return model.Int64Value(int64(index * 10))
			},
			options: model.CompressionOptions{Int: "plain"},
			check: func(t *testing.T, sample model.VersionedSample) {
				t.Helper()
				if sample.Value.Int64 != 50 {
					t.Fatalf("int value = %d, want 50", sample.Value.Int64)
				}
			},
		},
		{
			name:      "bool",
			fieldType: model.FieldBool,
			value: func(index int) model.FieldValue {
				return model.BoolValue(index%2 == 1)
			},
			options: model.CompressionOptions{},
			check: func(t *testing.T, sample model.VersionedSample) {
				t.Helper()
				if !sample.Value.Bool {
					t.Fatal("bool value = false, want true")
				}
			},
		},
		{
			name:      "string",
			fieldType: model.FieldString,
			value: func(index int) model.FieldValue {
				return model.StringValue(fmt.Sprintf("value-%02d", index))
			},
			options: model.CompressionOptions{String: "plain"},
			check: func(t *testing.T, sample model.VersionedSample) {
				t.Helper()
				if sample.Value.String != "value-05" {
					t.Fatalf("string value = %q, want value-05", sample.Value.String)
				}
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			column := model.ColumnData{
				FieldID:   13,
				FieldType: tt.fieldType,
				Samples:   makeSamples(tt.fieldType, 16, tt.value),
			}
			tt.options.Enabled = true
			tt.options.MinPageValues = 1
			tt.options.Timestamp = "plain"
			payload, err := marshalCompressedValueBlock(nil, column, tt.options)
			if err != nil {
				t.Fatalf("marshalCompressedValueBlock() error = %v", err)
			}
			got, err := unmarshalCompressedValueBlock(payload, Query{Start: 5, End: 5})
			if err != nil {
				t.Fatalf("unmarshalCompressedValueBlock() error = %v", err)
			}
			if len(got.Samples) != 1 {
				t.Fatalf("sample count = %d, want 1", len(got.Samples))
			}
			if got.Samples[0].Timestamp != 5 || got.Samples[0].WriteSeq != 6 {
				t.Fatalf("sample metadata = %#v, want timestamp 5 write seq 6", got.Samples[0])
			}
			tt.check(t, got.Samples[0])
		})
	}
}

func TestCompressedPlainStreamingRejectsMalformedPayloads(t *testing.T) {
	if got := compressedQueryCapacity(10, Query{Start: 10, End: 1}); got != 0 {
		t.Fatalf("compressedQueryCapacity(reversed) = %d, want 0", got)
	}
	if got := timestampPayloadCapacity(0); got != 0 {
		t.Fatalf("timestampPayloadCapacity(0) = %d, want 0", got)
	}
	if got := compressedQueryCapacity(10, Query{Start: 0, End: 100}); got != 10 {
		t.Fatalf("compressedQueryCapacity(wide) = %d, want 10", got)
	}
	_, err := readPlainCompressedSamples(
		model.FieldBool,
		9,
		appendPlainTimestamps(nil, []int64{0, 1, 2, 3, 4, 5, 6, 7, 8}),
		appendWriteSeqsPayload(nil, []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9})[2:],
		[]byte{0xff},
		Query{Start: 0, End: 8},
	)
	if err == nil {
		t.Fatal("readPlainCompressedSamples(truncated bool) error = nil, want error")
	}
	_, err = readPlainCompressedSamples(
		model.FieldType(99),
		1,
		appendPlainTimestamps(nil, []int64{0}),
		appendWriteSeqsPayload(nil, []uint64{1})[2:],
		[]byte{0},
		Query{Start: 0, End: 0},
	)
	if err == nil {
		t.Fatal("readPlainCompressedSamples(unsupported type) error = nil, want error")
	}
	if _, err := decodeCodecTimestamps(99, nil, 1); err == nil {
		t.Fatal("decodeCodecTimestamps(unknown) error = nil, want error")
	}
	if _, err := readXORFloatSampleValues(newBlockReader(nil), compressionPlain, []int64{1}, []uint64{1}, Query{Start: 0, End: 2}); err == nil {
		t.Fatal("readXORFloatSampleValues(wrong codec) error = nil, want error")
	}
	if _, err := readDeltaIntSampleValues(newBlockReader(nil), compressionPlain, []int64{1}, []uint64{1}, Query{Start: 0, End: 2}); err == nil {
		t.Fatal("readDeltaIntSampleValues(wrong codec) error = nil, want error")
	}
	if _, err := readDictionaryStringSampleValues(newBlockReader(nil), compressionPlain, []int64{1}, []uint64{1}, Query{Start: 0, End: 2}); err == nil {
		t.Fatal("readDictionaryStringSampleValues(wrong codec) error = nil, want error")
	}
	badDict := []byte{1, 1, 'a', 1}
	if _, err := readDictionaryStringSampleValues(newBlockReader(badDict), compressionDictionary, []int64{1}, []uint64{1}, Query{Start: 0, End: 2}); err == nil {
		t.Fatal("readDictionaryStringSampleValues(bad ordinal) error = nil, want error")
	}
}

func TestReadCodecSamplesPlainFallbackAllTypes(t *testing.T) {
	timestamps := []int64{1, 2, 3}
	writeSeqs := []uint64{10, 20, 30}
	cases := []struct {
		name      string
		fieldType model.FieldType
		column    model.ColumnData
	}{
		{
			name:      "float",
			fieldType: model.FieldFloat64,
			column: model.ColumnData{
				FieldType: model.FieldFloat64,
				Samples: makeSamples(model.FieldFloat64, 3, func(index int) model.FieldValue {
					return model.Float64Value(float64(index) + 0.25)
				}),
			},
		},
		{
			name:      "int",
			fieldType: model.FieldInt64,
			column: model.ColumnData{
				FieldType: model.FieldInt64,
				Samples: makeSamples(model.FieldInt64, 3, func(index int) model.FieldValue {
					return model.Int64Value(int64(index + 100))
				}),
			},
		},
		{
			name:      "bool",
			fieldType: model.FieldBool,
			column: model.ColumnData{
				FieldType: model.FieldBool,
				Samples: makeSamples(model.FieldBool, 3, func(index int) model.FieldValue {
					return model.BoolValue(index == 1)
				}),
			},
		},
		{
			name:      "string",
			fieldType: model.FieldString,
			column: model.ColumnData{
				FieldType: model.FieldString,
				Samples: makeSamples(model.FieldString, 3, func(index int) model.FieldValue {
					return model.StringValue(fmt.Sprintf("s%d", index))
				}),
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			values, err := appendSampleValues(nil, tt.column)
			if err != nil {
				t.Fatalf("appendSampleValues() error = %v", err)
			}
			reader := newBlockReader(appendCodecPayload(nil, compressionPlain, values))
			samples, err := readCodecSamples(reader, tt.fieldType, timestamps, writeSeqs, Query{Start: 2, End: 2})
			if err != nil {
				t.Fatalf("readCodecSamples() error = %v", err)
			}
			if err := reader.done("codec samples"); err != nil {
				t.Fatalf("reader.done() error = %v", err)
			}
			if len(samples) != 1 || samples[0].Timestamp != 2 || samples[0].WriteSeq != 20 {
				t.Fatalf("samples = %#v, want timestamp 2 write seq 20", samples)
			}
			if !reflect.DeepEqual(samples[0].Value, tt.column.Samples[1].Value) {
				t.Fatalf("value = %#v, want %#v", samples[0].Value, tt.column.Samples[1].Value)
			}
		})
	}
}

func TestReadCodecSamplesCompressedFallback(t *testing.T) {
	column := model.ColumnData{
		FieldType: model.FieldInt64,
		Samples: makeSamples(model.FieldInt64, 4, func(index int) model.FieldValue {
			return model.Int64Value(1_000 + int64(index))
		}),
	}
	codecID, payload, err := encodeIntValues(column.Samples, "delta")
	if err != nil {
		t.Fatalf("encodeIntValues() error = %v", err)
	}
	if codecID != compressionDelta {
		t.Fatalf("codecID = %d, want delta", codecID)
	}
	reader := newBlockReader(appendCodecPayload(nil, codecID, payload))
	samples, err := readCodecSamples(
		reader,
		model.FieldInt64,
		[]int64{1, 2, 3, 4},
		[]uint64{10, 20, 30, 40},
		Query{Start: 2, End: 3},
	)
	if err != nil {
		t.Fatalf("readCodecSamples() error = %v", err)
	}
	if err := reader.done("codec samples"); err != nil {
		t.Fatalf("reader.done() error = %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("sample count = %d, want 2", len(samples))
	}
	if samples[0].Value.Int64 != 1_001 || samples[1].Value.Int64 != 1_002 {
		t.Fatalf("samples = %#v, want compressed int values 1001 and 1002", samples)
	}
}

func TestCompressedValueReaderBoundaries(t *testing.T) {
	floatSamples := makeSamples(model.FieldFloat64, 3, func(index int) model.FieldValue {
		return model.Float64Value(float64(index) + 0.5)
	})
	floatPayload := appendXORFloatValues(nil, floatSamples)
	floatValues, err := readXORFloatValues(newBlockReader(floatPayload), compressionXOR, len(floatSamples))
	if err != nil {
		t.Fatalf("readXORFloatValues() error = %v", err)
	}
	if len(floatValues) != 3 || floatValues[2].Float64 != 2.5 {
		t.Fatalf("float values = %#v, want decoded xor values", floatValues)
	}
	if _, err := readXORFloatValues(newBlockReader(floatPayload), compressionPlain, len(floatSamples)); err == nil {
		t.Fatal("readXORFloatValues(wrong codec) error = nil, want error")
	}
	if _, err := readXORFloatValues(newBlockReader(floatPayload[:1]), compressionXOR, len(floatSamples)); err == nil {
		t.Fatal("readXORFloatValues(truncated) error = nil, want error")
	}

	timestamps := []int64{1, 2}
	writeSeqs := []uint64{10, 20}
	if _, err := readCodecPayloadSamples(newBlockReader(nil), model.FieldBool, compressionXOR, timestamps, writeSeqs, Query{Start: 0, End: 3}); err == nil {
		t.Fatal("readCodecPayloadSamples(bool xor) error = nil, want error")
	}
	if _, err := readCompressedValues(newBlockReader(nil), model.FieldBool, compressionXOR, 1); err == nil {
		t.Fatal("readCompressedValues(bool xor) error = nil, want error")
	}

	irregular := makeSamples(model.FieldInt64, 4, func(index int) model.FieldValue {
		return model.Int64Value(int64(index))
	})
	irregular[0].Timestamp = 1
	irregular[1].Timestamp = 3
	irregular[2].Timestamp = 8
	irregular[3].Timestamp = 9
	codecID, payload, err := encodeSampleTimestamps(irregular, "delta-of-delta")
	if err != nil {
		t.Fatalf("encodeSampleTimestamps() error = %v", err)
	}
	gotTimes, err := decodeCodecTimestamps(codecID, payload, len(irregular))
	if err != nil {
		t.Fatalf("decodeCodecTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(gotTimes, []int64{1, 3, 8, 9}) {
		t.Fatalf("timestamps = %#v, want irregular timestamps", gotTimes)
	}
}

func TestCompressedCodecStreamingAllocations(t *testing.T) {
	column := model.ColumnData{
		FieldID:   21,
		FieldType: model.FieldInt64,
		Samples: makeSamples(model.FieldInt64, 128, func(index int) model.FieldValue {
			return model.Int64Value(1_000_000 + int64(index))
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "plain",
		Int:           "delta",
	})
	if err != nil {
		t.Fatalf("marshalCompressedValueBlock() error = %v", err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		got, err := unmarshalCompressedValueBlock(payload, Query{Start: 64, End: 64})
		if err != nil {
			t.Fatalf("unmarshalCompressedValueBlock() error = %v", err)
		}
		if len(got.Samples) != 1 || got.Samples[0].Value.Int64 != 1_000_064 {
			t.Fatalf("samples = %#v, want one delta-decoded int sample", got.Samples)
		}
	})
	if allocs > 3 {
		t.Fatalf("compressed delta int allocs/run = %.2f, want <= 3", allocs)
	}
}

func TestMarshalCompressedValueBlockAvoidsMetadataSplitAllocations(t *testing.T) {
	column := model.ColumnData{
		FieldID:   24,
		FieldType: model.FieldInt64,
		Samples: makeSamples(model.FieldInt64, 128, func(index int) model.FieldValue {
			return model.Int64Value(1000 + int64(index))
		}),
	}
	opts := model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "delta-of-delta",
		Int:           "delta",
	}
	dst := make([]byte, 0, 4096)
	allocs := testing.AllocsPerRun(100, func() {
		payload, err := marshalCompressedValueBlock(dst[:0], column, opts)
		if err != nil {
			t.Fatalf("marshalCompressedValueBlock() error = %v", err)
		}
		if len(payload) == 0 {
			t.Fatal("payload is empty")
		}
	})
	if allocs > 7 {
		t.Fatalf("marshal compressed page allocs/run = %.2f, want <= 7", allocs)
	}
}

func TestCompressedCodecStreamingCorrectness(t *testing.T) {
	cases := []struct {
		name    string
		column  model.ColumnData
		options model.CompressionOptions
		want    model.FieldValue
	}{
		{
			name: "xor float",
			column: model.ColumnData{
				FieldID:   22,
				FieldType: model.FieldFloat64,
				Samples: makeSamples(model.FieldFloat64, 32, func(index int) model.FieldValue {
					return model.Float64Value(42)
				}),
			},
			options: model.CompressionOptions{Float: "xor"},
			want:    model.Float64Value(42),
		},
		{
			name: "dictionary string",
			column: model.ColumnData{
				FieldID:   23,
				FieldType: model.FieldString,
				Samples: makeSamples(model.FieldString, 32, func(index int) model.FieldValue {
					return model.StringValue([]string{"alpha", "beta", "gamma"}[index%3])
				}),
			},
			options: model.CompressionOptions{String: "dictionary"},
			want:    model.StringValue("beta"),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.options.Enabled = true
			tt.options.MinPageValues = 1
			tt.options.Timestamp = "plain"
			payload, err := marshalCompressedValueBlock(nil, tt.column, tt.options)
			if err != nil {
				t.Fatalf("marshalCompressedValueBlock() error = %v", err)
			}
			got, err := unmarshalCompressedValueBlock(payload, Query{Start: 10, End: 10})
			if err != nil {
				t.Fatalf("unmarshalCompressedValueBlock() error = %v", err)
			}
			if len(got.Samples) != 1 {
				t.Fatalf("sample count = %d, want 1", len(got.Samples))
			}
			if !reflect.DeepEqual(got.Samples[0].Value, tt.want) {
				t.Fatalf("value = %#v, want %#v", got.Samples[0].Value, tt.want)
			}
		})
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
	if _, err := unmarshalCompressedValueBlock([]byte{valueEncodingPageCompressed}, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalCompressedValueBlock(short) error = nil, want error")
	}
	payload := []byte{
		valueEncodingPageCompressed,
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
	if _, err := readCompressedValueHeader(newBlockReader([]byte{valueEncodingPagePlain})); err == nil {
		t.Fatal("readCompressedValueHeader(wrong encoding) error = nil, want error")
	}
	payload := []byte{valueEncodingPageCompressed}
	payload = binary.AppendUvarint(payload, uint64(^uint32(0))+1)
	if _, err := readCompressedValueHeader(newBlockReader(payload)); err == nil {
		t.Fatal("readCompressedValueHeader(field overflow) error = nil, want error")
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

func sampleValues(samples []model.VersionedSample) []model.FieldValue {
	values := make([]model.FieldValue, len(samples))
	for index, sample := range samples {
		values[index] = sample.Value
	}
	return values
}
