package sstable

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestGorillaFloatRoundTripAndSmallerThanPlain(t *testing.T) {
	cases := []struct {
		name    string
		builder func(int) model.FieldValue
	}{
		{name: "constant", builder: func(int) model.FieldValue { return model.Float64Value(42) }},
		{name: "smooth", builder: func(i int) model.FieldValue { return model.Float64Value(100 + float64(i)*0.01) }},
		{name: "monotonic", builder: func(i int) model.FieldValue { return model.Float64Value(float64(i)) }},
		{name: "randomish", builder: func(i int) model.FieldValue {
			return model.Float64Value(math.Sin(float64(i)) * float64(i%7))
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			samples := makeSamples(model.FieldFloat64, 256, tt.builder)
			codecID, payload, err := encodeFloatValues(samples, "xor")
			if err != nil {
				t.Fatalf("encodeFloatValues() error = %v", err)
			}
			if codecID != compressionXOR && codecID != compressionPlain &&
				codecID != compressionConstStep && codecID != compressionRLE && codecID != compressionDelta {
				t.Fatalf("codec = %d, want xor/plain/const-step/rle/delta", codecID)
			}
			values, err := readCompressedValues(newBlockReader(payload), model.FieldFloat64, codecID, len(samples))
			if err != nil {
				t.Fatalf("readCompressedValues() error = %v", err)
			}
			for index, sample := range samples {
				if values[index].Float64 != sample.Value.Float64 {
					t.Fatalf("value[%d]=%v want %v", index, values[index].Float64, sample.Value.Float64)
				}
			}
			if tt.name == "constant" || tt.name == "smooth" {
				plain := appendFloatValues(nil, samples)
				if len(payload) >= len(plain) {
					t.Fatalf("%s gorilla size=%d not smaller than plain=%d", tt.name, len(payload), len(plain))
				}
			}
		})
	}
}

func TestGorillaFloatSampleQueryFilter(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 16, func(i int) model.FieldValue {
		return model.Float64Value(float64(i) + 0.5)
	})
	payload := appendGorillaFloatValues(nil, samples)
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for i := range samples {
		timestamps[i] = samples[i].Timestamp
		writeSeqs[i] = samples[i].WriteSeq
	}
	got, err := readGorillaFloatSampleValues(
		newBlockReader(payload),
		compressionXOR,
		timestamps,
		writeSeqs,
		Query{Start: 5, End: 8},
	)
	if err != nil {
		t.Fatalf("readGorillaFloatSampleValues() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len(got)=%d want 4", len(got))
	}
	// timestamps 5..8 对应 value = index+0.5 => 5.5..8.5
	if got[0].Value.Float64 != 5.5 || got[3].Value.Float64 != 8.5 {
		t.Fatalf("got=%#v", got)
	}
}

func TestConstStepTimestampRoundTrip(t *testing.T) {
	timestamps := make([]int64, 128)
	for i := range timestamps {
		timestamps[i] = int64(i) * 1_000
	}
	codecID, payload, err := encodeTimestamps(timestamps, "delta-of-delta")
	if err != nil {
		t.Fatalf("encodeTimestamps() error = %v", err)
	}
	if codecID != compressionConstStep {
		t.Fatalf("codec=%d want const-step", codecID)
	}
	// base(8) + step varint(通常 2 字节量级) 应远小于 plain
	if len(payload) > 16 {
		t.Fatalf("const-step payload too large: %d", len(payload))
	}
	got, err := decodeCodecTimestamps(codecID, payload, len(timestamps))
	if err != nil {
		t.Fatalf("decodeCodecTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("timestamps mismatch")
	}

	samples := makeSamples(model.FieldInt64, 64, func(i int) model.FieldValue {
		return model.Int64Value(int64(i))
	})
	for i := range samples {
		samples[i].Timestamp = int64(i)
	}
	codecID, payload, err = encodeSampleTimestamps(samples, "auto")
	if err != nil {
		t.Fatalf("encodeSampleTimestamps() error = %v", err)
	}
	if codecID != compressionConstStep {
		t.Fatalf("sample codec=%d want const-step", codecID)
	}
	got, err = decodeCodecTimestamps(codecID, payload, len(samples))
	if err != nil {
		t.Fatalf("decode sample timestamps error = %v", err)
	}
	want := make([]int64, len(samples))
	for i := range want {
		want[i] = int64(i)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sample timestamps=%v want %v", got, want)
	}
}

func TestConstStepFallsBackForIrregular(t *testing.T) {
	timestamps := []int64{1, 3, 8, 9, 20}
	codecID, payload, err := encodeTimestamps(timestamps, "delta-of-delta")
	if err != nil {
		t.Fatalf("encodeTimestamps() error = %v", err)
	}
	if codecID == compressionConstStep {
		t.Fatal("irregular series should not use const-step")
	}
	got, err := decodeCodecTimestamps(codecID, payload, len(timestamps))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("got=%v want=%v", got, timestamps)
	}
}

func TestIntDeltaRLERoundTripAndSmaller(t *testing.T) {
	// i0=index：全为 delta=1 长 run
	samples := makeSamples(model.FieldInt64, 512, func(i int) model.FieldValue {
		return model.Int64Value(int64(i))
	})
	codecID, payload, err := encodeIntValues(samples, "delta")
	if err != nil {
		t.Fatalf("encodeIntValues() error = %v", err)
	}
	if codecID != compressionRLE {
		t.Fatalf("codec=%d want rle", codecID)
	}
	deltaPayload := appendDeltaIntValues(nil, samples)
	if len(payload) >= len(deltaPayload) {
		t.Fatalf("rle size=%d not smaller than delta=%d", len(payload), len(deltaPayload))
	}
	values, err := readDeltaRLEIntValues(newBlockReader(payload), codecID, len(samples))
	if err != nil {
		t.Fatalf("readDeltaRLEIntValues() error = %v", err)
	}
	for i, sample := range samples {
		if values[i].Int64 != sample.Value.Int64 {
			t.Fatalf("value[%d]=%d want %d", i, values[i].Int64, sample.Value.Int64)
		}
	}

	// 交替 delta，RLE 不应优于 delta
	noisy := makeSamples(model.FieldInt64, 64, func(i int) model.FieldValue {
		if i%2 == 0 {
			return model.Int64Value(int64(i * 1000))
		}
		return model.Int64Value(int64(i))
	})
	codecID, payload, err = encodeIntValues(noisy, "delta")
	if err != nil {
		t.Fatalf("encodeIntValues(noisy) error = %v", err)
	}
	got, err := readCompressedValues(newBlockReader(payload), model.FieldInt64, codecID, len(noisy))
	if err != nil {
		t.Fatalf("readCompressedValues(noisy) error = %v", err)
	}
	for i, sample := range noisy {
		if got[i].Int64 != sample.Value.Int64 {
			t.Fatalf("noisy[%d]=%d want %d", i, got[i].Int64, sample.Value.Int64)
		}
	}
}

func TestIntDeltaRLESampleFilter(t *testing.T) {
	samples := makeSamples(model.FieldInt64, 20, func(i int) model.FieldValue {
		return model.Int64Value(100 + int64(i))
	})
	payload := appendDeltaRLEIntValues(nil, samples)
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for i := range samples {
		timestamps[i] = samples[i].Timestamp
		writeSeqs[i] = samples[i].WriteSeq
	}
	got, err := readDeltaRLEIntSampleValues(
		newBlockReader(payload),
		compressionRLE,
		timestamps,
		writeSeqs,
		Query{Start: 10, End: 12},
	)
	if err != nil {
		t.Fatalf("readDeltaRLEIntSampleValues() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	// timestamps 10..12 对应 value = 100+index => 110..112
	if got[0].Value.Int64 != 110 || got[2].Value.Int64 != 112 {
		t.Fatalf("got=%#v", got)
	}
}

func TestBitWriterReaderRoundTrip(t *testing.T) {
	writer := newBitWriter(nil)
	writer.writeBit(1)
	writer.writeBits(0b1011, 4)
	writer.writeBits(0, 3)
	writer.writeBits(0x3f, 6)
	data := writer.bytes()
	reader := newBitReader(data)
	bit, err := reader.readBit()
	if err != nil || bit != 1 {
		t.Fatalf("bit=%d err=%v", bit, err)
	}
	val, err := reader.readBits(4)
	if err != nil || val != 0b1011 {
		t.Fatalf("val=%d err=%v", val, err)
	}
	val, err = reader.readBits(3)
	if err != nil || val != 0 {
		t.Fatalf("val=%d err=%v", val, err)
	}
	val, err = reader.readBits(6)
	if err != nil || val != 0x3f {
		t.Fatalf("val=%d err=%v", val, err)
	}
}

func TestCompressedValueBlockWithP1Codecs(t *testing.T) {
	column := model.ColumnData{
		FieldID:   7,
		FieldType: model.FieldFloat64,
		Samples: makeSamples(model.FieldFloat64, 64, func(i int) model.FieldValue {
			return model.Float64Value(10 + float64(i)*0.5)
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "delta-of-delta",
		Float:         "xor",
	})
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	got, err := unmarshalCompressedValueBlock(payload, Query{Start: 0, End: 1 << 62})
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(got.Samples) != 64 {
		t.Fatalf("len=%d want 64", len(got.Samples))
	}
	if got.Samples[10].Value.Float64 != 15 {
		t.Fatalf("sample10=%v want 15", got.Samples[10].Value.Float64)
	}

	intColumn := model.ColumnData{
		FieldID:   8,
		FieldType: model.FieldInt64,
		Samples: makeSamples(model.FieldInt64, 128, func(i int) model.FieldValue {
			return model.Int64Value(int64(i))
		}),
	}
	payload, err = marshalCompressedValueBlock(nil, intColumn, model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "delta-of-delta",
		Int:           "delta",
	})
	if err != nil {
		t.Fatalf("marshal int error = %v", err)
	}
	got, err = unmarshalCompressedValueBlock(payload, Query{Start: 0, End: 1 << 62})
	if err != nil {
		t.Fatalf("unmarshal int error = %v", err)
	}
	if len(got.Samples) != 128 || got.Samples[50].Value.Int64 != 50 {
		t.Fatalf("int samples wrong: %#v", got.Samples[50])
	}
}

func TestWriteSeqDeltaRLERoundTrip(t *testing.T) {
	samples := makeSamples(model.FieldInt64, 256, func(i int) model.FieldValue {
		return model.Int64Value(int64(i))
	})
	// makeSamples 的 WriteSeq 为 index+1 递增，应走 RLE
	codecID, payload := encodeWriteSeqs(samples)
	if codecID != compressionRLE {
		t.Fatalf("codec=%d want rle", codecID)
	}
	got, err := decodeWriteSeqs(codecID, payload, len(samples))
	if err != nil {
		t.Fatalf("decodeWriteSeqs() error = %v", err)
	}
	for i, sample := range samples {
		if got[i] != sample.WriteSeq {
			t.Fatalf("seq[%d]=%d want %d", i, got[i], sample.WriteSeq)
		}
	}
	plain := appendPlainWriteSeqs(nil, samples)
	if len(payload) >= len(plain) {
		t.Fatalf("rle=%d not smaller than plain=%d", len(payload), len(plain))
	}

	// 随机 seq 回退 plain
	noisy := make([]model.VersionedSample, 32)
	for i := range noisy {
		noisy[i].WriteSeq = uint64((i*37 + 11) % 97)
	}
	codecID, payload = encodeWriteSeqs(noisy)
	got, err = decodeWriteSeqs(codecID, payload, len(noisy))
	if err != nil {
		t.Fatalf("decode noisy error = %v", err)
	}
	for i := range noisy {
		if got[i] != noisy[i].WriteSeq {
			t.Fatalf("noisy[%d]=%d want %d", i, got[i], noisy[i].WriteSeq)
		}
	}
}

func TestGorillaFloatErrorPaths(t *testing.T) {
	if _, err := readGorillaFloatValues(newBlockReader(nil), compressionPlain, 1); err == nil {
		t.Fatal("wrong codec error = nil")
	}
	if _, err := readGorillaFloatValues(newBlockReader([]byte{1, 2, 3}), compressionXOR, 1); err == nil {
		t.Fatal("truncated first bits error = nil")
	}
	// truncated after first value
	samples := makeSamples(model.FieldFloat64, 4, func(i int) model.FieldValue {
		return model.Float64Value(float64(i))
	})
	payload := appendGorillaFloatValues(nil, samples)
	if _, err := readGorillaFloatValues(newBlockReader(payload[:8]), compressionXOR, 4); err == nil {
		t.Fatal("truncated gorilla stream error = nil")
	}
	if values, err := readGorillaFloatValues(newBlockReader(payload[:8]), compressionXOR, 1); err != nil || len(values) != 1 {
		t.Fatalf("single value decode = %v, %v", values, err)
	}
	if _, err := readGorillaFloatSampleValues(newBlockReader(nil), compressionPlain, []int64{1}, []uint64{1}, Query{}); err == nil {
		t.Fatal("sample wrong codec error = nil")
	}
	if samplesOut, err := readGorillaFloatSampleValues(newBlockReader(nil), compressionXOR, nil, nil, Query{}); err != nil || len(samplesOut) != 0 {
		t.Fatalf("empty timestamps = %v %v", samplesOut, err)
	}
}

func TestConstStepTimestampErrorPaths(t *testing.T) {
	if _, err := decodeConstStepTimestamps([]byte{1, 2, 3}, 2); err == nil {
		t.Fatal("truncated const-step error = nil")
	}
	if got, err := decodeConstStepTimestamps(nil, 0); err != nil || got != nil {
		t.Fatalf("empty count = %v %v", got, err)
	}
	// base only, missing step
	baseOnly := make([]byte, 8)
	if _, err := decodeConstStepTimestamps(baseOnly, 2); err == nil {
		t.Fatal("missing step error = nil")
	}
	if _, err := decodeCodecTimestamps(99, nil, 1); err == nil {
		t.Fatal("unknown codec error = nil")
	}
}

func TestIntRLEErrorPaths(t *testing.T) {
	if _, err := readDeltaRLEIntValues(newBlockReader(nil), compressionDelta, 1); err == nil {
		t.Fatal("wrong codec error = nil")
	}
	if _, err := readDeltaRLEIntValues(newBlockReader(nil), compressionRLE, 1); err == nil {
		t.Fatal("missing first value error = nil")
	}
	// first value ok, run=0
	payload := []byte{}
	payload = appendVarintForTest(payload, 10)
	payload = append(payload, 0) // run uvarint 0
	if _, err := readDeltaRLEIntValues(newBlockReader(payload), compressionRLE, 2); err == nil {
		t.Fatal("zero run error = nil")
	}
	// overflow run
	payload = appendVarintForTest(nil, 1)
	payload = appendUvarintForTest(payload, 5) // run too large
	payload = appendUvarintForTest(payload, 0) // delta 0
	if _, err := readDeltaRLEIntValues(newBlockReader(payload), compressionRLE, 2); err == nil {
		t.Fatal("overflow run error = nil")
	}
	if samples, err := readDeltaRLEIntSampleValues(newBlockReader(nil), compressionRLE, nil, nil, Query{}); err != nil || len(samples) != 0 {
		t.Fatalf("empty sample rle = %v %v", samples, err)
	}
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(nil), compressionDelta, []int64{1}, []uint64{1}, Query{}); err == nil {
		t.Fatal("sample wrong codec error = nil")
	}
}

func TestWriteSeqRLEErrorPaths(t *testing.T) {
	if _, err := decodeWriteSeqs(99, nil, 1); err == nil {
		t.Fatal("unknown write seq codec error = nil")
	}
	if _, err := decodeDeltaRLEWriteSeqs(nil, 1); err == nil {
		t.Fatal("missing first seq error = nil")
	}
	payload := appendUvarintForTest(nil, 1)
	payload = append(payload, 0) // run 0
	if _, err := decodeDeltaRLEWriteSeqs(payload, 2); err == nil {
		t.Fatal("zero run error = nil")
	}
	payload = appendUvarintForTest(nil, 1)
	payload = appendUvarintForTest(payload, 5)
	payload = appendUvarintForTest(payload, 0)
	if _, err := decodeDeltaRLEWriteSeqs(payload, 2); err == nil {
		t.Fatal("overflow run error = nil")
	}
	if got, err := decodeDeltaRLEWriteSeqs(nil, 0); err != nil || got != nil {
		t.Fatalf("empty count = %v %v", got, err)
	}
	// single sample encode uses plain
	samples := makeSamples(model.FieldInt64, 1, func(int) model.FieldValue { return model.Int64Value(1) })
	codec, payload := encodeWriteSeqs(samples)
	if codec != compressionPlain {
		t.Fatalf("single seq codec=%d want plain", codec)
	}
	got, err := decodeWriteSeqs(codec, payload, 1)
	if err != nil || got[0] != 1 {
		t.Fatalf("single decode = %v %v", got, err)
	}
}

func TestDeltaIntStillRoundTrips(t *testing.T) {
	// 不规则 delta，确保 delta 路径仍可用
	samples := makeSamples(model.FieldInt64, 16, func(i int) model.FieldValue {
		return model.Int64Value(int64(i*i - 3*i))
	})
	payload := appendDeltaIntValues(nil, samples)
	values, err := readDeltaIntValues(newBlockReader(payload), compressionDelta, len(samples))
	if err != nil {
		t.Fatalf("readDeltaIntValues() error = %v", err)
	}
	for i, sample := range samples {
		if values[i].Int64 != sample.Value.Int64 {
			t.Fatalf("value[%d]=%d want %d", i, values[i].Int64, sample.Value.Int64)
		}
	}
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for i := range samples {
		timestamps[i] = samples[i].Timestamp
		writeSeqs[i] = samples[i].WriteSeq
	}
	got, err := readDeltaIntSampleValues(newBlockReader(payload), compressionDelta, timestamps, writeSeqs, Query{Start: 0, End: 100})
	if err != nil || len(got) != len(samples) {
		t.Fatalf("sample delta = %d err=%v", len(got), err)
	}
}

func appendVarintForTest(dst []byte, value int64) []byte {
	return append(dst, encodeVarintForTest(value)...)
}

func appendUvarintForTest(dst []byte, value uint64) []byte {
	return append(dst, encodeUvarintForTest(value)...)
}

func encodeVarintForTest(value int64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, value)
	return buf[:n]
}

func encodeUvarintForTest(value uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, value)
	return buf[:n]
}

func TestDeltaOfDeltaEqualRunStillRoundTrips(t *testing.T) {
	// 直接调用 DoD 编码器，覆盖 const-step 抢走后的等 delta run 路径。
	timestamps := make([]int64, 64)
	for i := range timestamps {
		timestamps[i] = int64(i) * 1000
	}
	payload := appendDeltaOfDeltaTimestamps(nil, timestamps)
	got, err := decodeDeltaOfDeltaTimestamps(payload, len(timestamps))
	if err != nil {
		t.Fatalf("decodeDeltaOfDeltaTimestamps() error = %v", err)
	}
	if !reflect.DeepEqual(got, timestamps) {
		t.Fatalf("timestamps mismatch")
	}
	samples := makeSamples(model.FieldInt64, 32, func(i int) model.FieldValue {
		return model.Int64Value(int64(i))
	})
	for i := range samples {
		samples[i].Timestamp = int64(i) * 5
	}
	payload = appendDeltaOfDeltaSampleTimestamps(nil, samples)
	got, err = decodeDeltaOfDeltaTimestamps(payload, len(samples))
	if err != nil {
		t.Fatalf("decode sample DoD error = %v", err)
	}
	want := make([]int64, len(samples))
	for i := range want {
		want[i] = int64(i) * 5
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sample timestamps mismatch")
	}
}

func TestGorillaReuseAndBitEdges(t *testing.T) {
	// 构造可复用 leading/trailing 的序列：相邻值仅在相同 bit 窗口变化
	samples := make([]model.VersionedSample, 8)
	base := math.Float64frombits(0x4000000000000000) // 2.0
	samples[0] = model.VersionedSample{Timestamp: 0, WriteSeq: 1, Value: model.Float64Value(base)}
	// 逐步改动低位 mantissa
	bits := math.Float64bits(base)
	for i := 1; i < len(samples); i++ {
		bits ^= 0x3 // flip low bits within same trailing window often
		samples[i] = model.VersionedSample{
			Timestamp: int64(i),
			WriteSeq:  uint64(i + 1),
			Value:     model.Float64Value(math.Float64frombits(bits)),
		}
	}
	payload := appendGorillaFloatValues(nil, samples)
	values, err := readGorillaFloatValues(newBlockReader(payload), compressionXOR, len(samples))
	if err != nil {
		t.Fatalf("read gorilla error = %v", err)
	}
	for i := range samples {
		if values[i].Float64 != samples[i].Value.Float64 {
			t.Fatalf("value[%d]=%v want %v", i, values[i].Float64, samples[i].Value.Float64)
		}
	}
	// bit writer/reader edges
	writer := newBitWriter(nil)
	writer.writeBits(0, 0)
	if len(writer.bytes()) != 0 {
		t.Fatalf("empty bits should keep dst empty")
	}
	writer = newBitWriter(nil)
	writer.writeBit(1)
	data := writer.bytes()
	reader := newBitReader(data)
	if _, err := reader.readBits(0); err != nil {
		t.Fatalf("readBits(0) error = %v", err)
	}
	if _, err := reader.readBits(9); err == nil {
		t.Fatal("over-read error = nil")
	}
	// consumeAligned with short rest should fail
	br := &blockReader{rest: []byte{0x80}}
	bitR := newBitReader(bytes.Repeat([]byte{0xff}, 2))
	bitR.pos = 9
	if err := bitR.consumeAligned(br); err == nil {
		t.Fatal("consumeAligned short rest error = nil")
	}
}

func TestCompressionPolicyAliases(t *testing.T) {
	if got := compressionPolicy("gorilla", "xor"); got != "xor" {
		t.Fatalf("gorilla alias = %q", got)
	}
	if got := compressionPolicy("rle", "delta"); got != "delta" {
		t.Fatalf("rle alias = %q", got)
	}
	if got := compressionPolicy("gorilla", "delta"); got != "delta" {
		t.Fatalf("gorilla non-float default = %q", got)
	}
	if got := compressionPolicy("weird", "xor"); got != "xor" {
		t.Fatalf("unknown policy = %q", got)
	}
	codec, _, err := encodeFloatValues(makeSamples(model.FieldFloat64, 16, func(int) model.FieldValue {
		return model.Float64Value(1)
	}), "gorilla")
	if err != nil {
		t.Fatalf("encode gorilla policy err=%v", err)
	}
	if codec != compressionXOR && codec != compressionPlain &&
		codec != compressionConstStep && codec != compressionRLE && codec != compressionDelta {
		t.Fatalf("encode gorilla policy codec=%d", codec)
	}
}

func TestWriteSeqAndIntRLEMissingDelta(t *testing.T) {
	// first + run without delta
	payload := appendUvarintForTest(nil, 10)
	payload = appendUvarintForTest(payload, 1)
	if _, err := decodeDeltaRLEWriteSeqs(payload, 2); err == nil {
		t.Fatal("missing writeSeq delta error = nil")
	}
	payload = appendVarintForTest(nil, 10)
	payload = appendUvarintForTest(payload, 1)
	if _, err := readDeltaRLEIntValues(newBlockReader(payload), compressionRLE, 2); err == nil {
		t.Fatal("missing int delta error = nil")
	}
	// sample path missing first
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(nil), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample missing first error = nil")
	}
}

func TestP1CoverageBoosters(t *testing.T) {
	// empty gorilla encode
	if got := appendGorillaFloatValues(nil, nil); len(got) != 0 {
		t.Fatalf("empty gorilla encode = %v", got)
	}
	// single sample gorilla sample path
	single := makeSamples(model.FieldFloat64, 1, func(int) model.FieldValue { return model.Float64Value(3.14) })
	payload := appendGorillaFloatValues(nil, single)
	samples, err := readGorillaFloatSampleValues(
		newBlockReader(payload),
		compressionXOR,
		[]int64{single[0].Timestamp},
		[]uint64{single[0].WriteSeq},
		Query{Start: 0, End: 10},
	)
	if err != nil || len(samples) != 1 || samples[0].Value.Float64 != 3.14 {
		t.Fatalf("single sample gorilla = %#v err=%v", samples, err)
	}
	// truncated first on sample path
	if _, err := readGorillaFloatSampleValues(newBlockReader([]byte{1}), compressionXOR, []int64{1, 2}, []uint64{1, 2}, Query{}); err == nil {
		t.Fatal("truncated sample first error = nil")
	}
	// truncated stream on sample path after first
	many := makeSamples(model.FieldFloat64, 5, func(i int) model.FieldValue { return model.Float64Value(float64(i) + 0.25) })
	payload = appendGorillaFloatValues(nil, many)
	if _, err := readGorillaFloatSampleValues(newBlockReader(payload[:8]), compressionXOR, []int64{0, 1, 2, 3, 4}, []uint64{1, 2, 3, 4, 5}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("truncated sample stream error = nil")
	}
	// gorilla invalid reuse before block via handcrafted stream: first 8 bytes + bit "10" (control=1,reuse=0) without prior block
	hand := binary.LittleEndian.AppendUint64(nil, math.Float64bits(1.0))
	// write bits 1,0 and pad
	bw := newBitWriter(hand)
	bw.writeBit(1)
	bw.writeBit(0)
	hand = bw.bytes()
	if _, err := readGorillaFloatValues(newBlockReader(hand), compressionXOR, 2); err == nil {
		t.Fatal("reuse-before-block error = nil")
	}
	// invalid window: control=1,reuse=1,leading=0,mbits-1=63 => mbits=64, trailing=0 ok; use leading=20 mbits=50 => trailing=-6 invalid
	hand = binary.LittleEndian.AppendUint64(nil, math.Float64bits(1.0))
	bw = newBitWriter(hand)
	bw.writeBit(1)      // control
	bw.writeBit(1)      // new block
	bw.writeBits(20, 5) // leading
	bw.writeBits(49, 6) // mbits-1 = 49 => mbits=50, trailing=-6
	hand = bw.bytes()
	if _, err := readGorillaFloatValues(newBlockReader(hand), compressionXOR, 2); err == nil {
		t.Fatal("invalid window error = nil")
	}
	// consumeAligned when usedBytes > len(data)
	br := &blockReader{rest: bytes.Repeat([]byte{1}, 8)}
	bitR := newBitReader([]byte{0xff})
	bitR.pos = 100
	if err := bitR.consumeAligned(br); err == nil {
		t.Fatal("consumeAligned oversize error = nil")
	}
	// writeSeq decode plain via decodeWriteSeqs
	seqs := makeSamples(model.FieldInt64, 3, func(i int) model.FieldValue { return model.Int64Value(int64(i)) })
	// force non-monotonic seqs so plain wins
	seqs[0].WriteSeq = 100
	seqs[1].WriteSeq = 1
	seqs[2].WriteSeq = 50
	codec, payloadSeq := encodeWriteSeqs(seqs)
	got, err := decodeWriteSeqs(codec, payloadSeq, 3)
	if err != nil || got[0] != 100 || got[2] != 50 {
		t.Fatalf("plain writeSeq decode = %v err=%v codec=%d", got, err, codec)
	}
	// int rle sample overflow/missing run
	payload = appendVarintForTest(nil, 1)
	payload = appendUvarintForTest(payload, 2)
	payload = appendUvarintForTest(payload, 0)
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(payload), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample run overflow boundary error = nil")
	}
	// craft overflow explicitly count=2, first + run3
	payload = appendVarintForTest(nil, 1)
	payload = appendUvarintForTest(payload, 3)
	payload = appendUvarintForTest(payload, 0)
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(payload), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample overflow error = nil")
	}
	// missing run on sample path
	payload = appendVarintForTest(nil, 1)
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(payload), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample missing run error = nil")
	}
	// zero run sample path
	payload = appendVarintForTest(nil, 1)
	payload = append(payload, 0)
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(payload), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample zero run error = nil")
	}
	// missing delta sample path
	payload = appendVarintForTest(nil, 1)
	payload = appendUvarintForTest(payload, 1)
	if _, err := readDeltaRLEIntSampleValues(newBlockReader(payload), compressionRLE, []int64{1, 2}, []uint64{1, 2}, Query{Start: 0, End: 10}); err == nil {
		t.Fatal("sample missing delta error = nil")
	}
	// empty int rle values
	if values, err := readDeltaRLEIntValues(newBlockReader(nil), compressionRLE, 0); err != nil || len(values) != 0 {
		t.Fatalf("empty int rle = %v %v", values, err)
	}
	// encodeIntValues empty / single
	if codec, payload, err := encodeIntValues(nil, "delta"); err != nil || len(payload) != 0 {
		t.Fatalf("empty int encode codec=%d payload=%v err=%v", codec, payload, err)
	}
	if codec, payload, err := encodeIntValues(makeSamples(model.FieldInt64, 1, func(int) model.FieldValue { return model.Int64Value(9) }), "delta"); err != nil {
		t.Fatalf("single int encode err=%v", err)
	} else if _, err := readCompressedValues(newBlockReader(payload), model.FieldInt64, codec, 1); err != nil {
		t.Fatalf("single int decode err=%v", err)
	}
	// encode float empty
	if codec, payload, err := encodeFloatValues(nil, "xor"); err != nil || len(payload) != 0 {
		t.Fatalf("empty float encode = %d %v %v", codec, payload, err)
	}
	// const step single uses plain-ish path via encodeTimestamps len<2 => not const
	if codec, _, err := encodeTimestamps([]int64{42}, "delta-of-delta"); err != nil {
		t.Fatalf("single ts err=%v", err)
	} else if codec != compressionPlain && codec != compressionDeltaOfDelta && codec != compressionConstStep {
		t.Fatalf("unexpected codec %d", codec)
	}
	// writeSeq empty
	if codec, payload := encodeWriteSeqs(nil); codec != compressionPlain || len(payload) != 0 {
		t.Fatalf("empty writeSeq = %d %v", codec, payload)
	}
}

func TestPlainCompressedTypedPaths(t *testing.T) {
	// 直接覆盖 all-plain 快路径（writeSeq RLE 后该路径更少被自动命中）。
	makeTime := func(n int) []byte {
		ts := make([]int64, n)
		for i := range ts {
			ts[i] = int64(i)
		}
		return appendPlainTimestamps(nil, ts)
	}
	makeSeqPlain := func(n int) []byte {
		samples := make([]model.VersionedSample, n)
		// 非单调，避免 RLE
		for i := range samples {
			samples[i].WriteSeq = uint64((i*13 + 7) % 97)
		}
		return appendPlainWriteSeqs(nil, samples)
	}

	// float plain values
	floatSamples := makeSamples(model.FieldFloat64, 8, func(i int) model.FieldValue {
		return model.Float64Value(float64(i) + 0.25)
	})
	floatVals := appendFloatValues(nil, floatSamples)
	got, err := readPlainCompressedSamples(model.FieldFloat64, 8, makeTime(8), makeSeqPlain(8), floatVals, Query{Start: 2, End: 4})
	if err != nil || len(got) != 3 {
		t.Fatalf("plain float samples = %#v err=%v", got, err)
	}
	if got[0].Value.Float64 != 2.25 {
		t.Fatalf("float value = %v", got[0].Value.Float64)
	}

	// bool plain
	boolSamples := makeSamples(model.FieldBool, 8, func(i int) model.FieldValue {
		return model.BoolValue(i%2 == 0)
	})
	boolVals, err := appendBoolValues(nil, boolSamples)
	if err != nil {
		t.Fatalf("appendBoolValues() error = %v", err)
	}
	got, err = readPlainCompressedSamples(model.FieldBool, 8, makeTime(8), makeSeqPlain(8), boolVals, Query{Start: 0, End: 7})
	if err != nil || len(got) != 8 {
		t.Fatalf("plain bool samples = %#v err=%v", got, err)
	}
	if !got[0].Value.Bool || got[1].Value.Bool {
		t.Fatalf("bool values wrong: %#v", got[:2])
	}

	// string plain
	stringSamples := makeSamples(model.FieldString, 4, func(i int) model.FieldValue {
		return model.StringValue("v" + string(rune('a'+i)))
	})
	stringVals := appendStringValues(nil, stringSamples)
	got, err = readPlainCompressedSamples(model.FieldString, 4, makeTime(4), makeSeqPlain(4), stringVals, Query{Start: 1, End: 2})
	if err != nil || len(got) != 2 {
		t.Fatalf("plain string samples = %#v err=%v", got, err)
	}
	if got[0].Value.String == "" {
		t.Fatalf("string empty: %#v", got)
	}

	// int plain already partially covered; exercise full window
	intSamples := makeSamples(model.FieldInt64, 6, func(i int) model.FieldValue {
		return model.Int64Value(int64(i * 3))
	})
	intVals := appendIntValues(nil, intSamples)
	got, err = readPlainCompressedSamples(model.FieldInt64, 6, makeTime(6), makeSeqPlain(6), intVals, Query{Start: 0, End: 5})
	if err != nil || len(got) != 6 || got[5].Value.Int64 != 15 {
		t.Fatalf("plain int samples = %#v err=%v", got, err)
	}

	// marshal/unmarshal with forced plain policies to hit plain sample path through public API
	// use noisy writeSeq so writeSeq codec stays plain
	column := model.ColumnData{FieldID: 1, FieldType: model.FieldFloat64, Samples: floatSamples}
	for i := range column.Samples {
		column.Samples[i].WriteSeq = uint64((i*17 + 3) % 89)
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled: true, MinPageValues: 1,
		Timestamp: "plain", Float: "plain",
	})
	if err != nil {
		t.Fatalf("marshal plain float error = %v", err)
	}
	block, err := unmarshalCompressedValueBlock(payload, Query{Start: 0, End: 100})
	if err != nil || len(block.Samples) != 8 {
		t.Fatalf("unmarshal plain float = %#v err=%v", block.Samples, err)
	}
}
