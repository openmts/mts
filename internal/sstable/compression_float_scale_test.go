package sstable

import (
	"math"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestFloatConstStepRoundTripForScaleLikeSeries(t *testing.T) {
	cases := []struct {
		name string
		fn   func(int) float64
	}{
		{name: "f0", fn: func(i int) float64 { return float64(i) }},
		{name: "f1", fn: func(i int) float64 { return float64(i) * 1.1 }},
		{name: "f2", fn: func(i int) float64 { return float64(i) * 1.2 }},
		{name: "const", fn: func(int) float64 { return 42 }},
		{name: "offset", fn: func(i int) float64 { return 100 + float64(i)*0.5 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			samples := makeSamples(model.FieldFloat64, 256, func(i int) model.FieldValue {
				return model.Float64Value(tt.fn(i))
			})
			codec, payload, err := encodeFloatValues(samples, "xor")
			if err != nil {
				t.Fatalf("encode error = %v", err)
			}
			// 整数值等差可走 RLE（更短）；非整数等差走 const-step。
			if codec != compressionConstStep && codec != compressionRLE && codec != compressionDelta {
				t.Fatalf("codec=%d want const-step/rle/delta", codec)
			}
			if codec == compressionConstStep && len(payload) != 16 {
				t.Fatalf("const-step payload len=%d want 16", len(payload))
			}
			if len(payload) >= 256*8 {
				t.Fatalf("payload too large: %d", len(payload))
			}
			values, err := readCompressedFloatValues(newBlockReader(payload), codec, len(samples))
			if err != nil {
				t.Fatalf("decode error = %v", err)
			}
			for i, sample := range samples {
				if values[i].Float64 != sample.Value.Float64 {
					t.Fatalf("value[%d]=%v want %v", i, values[i].Float64, sample.Value.Float64)
				}
			}
		})
	}
}

func TestFloatIntegerPathUsesIntRLE(t *testing.T) {
	// 非整步但整数值：0,1,2,4,8,... 不是 const step
	samples := makeSamples(model.FieldFloat64, 32, func(i int) model.FieldValue {
		return model.Float64Value(float64(i * i))
	})
	codec, payload, err := encodeFloatValues(samples, "xor")
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	// 至少应优于 plain
	plain := appendFloatValues(nil, samples)
	if len(payload) >= len(plain) {
		t.Fatalf("payload=%d not < plain=%d codec=%d", len(payload), len(plain), codec)
	}
	values, err := readCompressedFloatValues(newBlockReader(payload), codec, len(samples))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	for i, sample := range samples {
		if values[i].Float64 != sample.Value.Float64 {
			t.Fatalf("value[%d]=%v want %v", i, values[i].Float64, sample.Value.Float64)
		}
	}
}

func TestFloatConstStepSampleFilter(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 20, func(i int) model.FieldValue {
		return model.Float64Value(float64(i) * 1.1)
	})
	payload := appendFloatConstStepValues(nil, 0, 1.1)
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for i := range samples {
		timestamps[i] = samples[i].Timestamp
		writeSeqs[i] = samples[i].WriteSeq
	}
	got, err := readFloatConstStepSampleValues(
		newBlockReader(payload),
		compressionConstStep,
		timestamps,
		writeSeqs,
		Query{Start: 5, End: 7},
	)
	if err != nil || len(got) != 3 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if got[0].Value.Float64 != float64(5)*1.1 || got[2].Value.Float64 != float64(7)*1.1 {
		t.Fatalf("values=%v %v", got[0].Value.Float64, got[2].Value.Float64)
	}
}

func TestFloatEncodeFallsBackForIrregular(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 32, func(i int) model.FieldValue {
		return model.Float64Value(math.Sin(float64(i)))
	})
	codec, payload, err := encodeFloatValues(samples, "xor")
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	values, err := readCompressedValues(newBlockReader(payload), model.FieldFloat64, codec, len(samples))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	for i, sample := range samples {
		if values[i].Float64 != sample.Value.Float64 {
			t.Fatalf("value[%d]=%v want %v", i, values[i].Float64, sample.Value.Float64)
		}
	}
}

func TestFloatConstStepErrorPaths(t *testing.T) {
	if _, err := readFloatConstStepValues(newBlockReader(nil), compressionXOR, 1); err == nil {
		t.Fatal("wrong codec error = nil")
	}
	if values, err := readFloatConstStepValues(newBlockReader(nil), compressionConstStep, 0); err != nil || values != nil {
		t.Fatalf("empty = %v %v", values, err)
	}
	if _, err := readFloatConstStepValues(newBlockReader([]byte{1, 2, 3}), compressionConstStep, 2); err == nil {
		t.Fatal("truncated error = nil")
	}
	if _, err := readCompressedFloatValues(newBlockReader(nil), 99, 1); err == nil {
		t.Fatal("unknown codec error = nil")
	}
	if samples, err := readFloatConstStepSampleValues(newBlockReader(nil), compressionConstStep, nil, nil, Query{}); err != nil || samples != nil {
		t.Fatalf("empty samples = %v %v", samples, err)
	}
}

func TestFloatSamplesAsIntRejectsNonInteger(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 4, func(i int) model.FieldValue {
		return model.Float64Value(float64(i) + 0.5)
	})
	if _, ok := floatSamplesAsIntSamples(samples); ok {
		t.Fatal("non-integer accepted")
	}
	samples = makeSamples(model.FieldFloat64, 4, func(i int) model.FieldValue {
		return model.Float64Value(math.NaN())
	})
	if _, ok := floatSamplesAsIntSamples(samples); ok {
		t.Fatal("nan accepted")
	}
}

func TestCompressedValueBlockFloatConstStep(t *testing.T) {
	column := model.ColumnData{
		FieldID:   9,
		FieldType: model.FieldFloat64,
		Samples: makeSamples(model.FieldFloat64, 64, func(i int) model.FieldValue {
			return model.Float64Value(float64(i) * 1.3)
		}),
	}
	payload, err := marshalCompressedValueBlock(nil, column, model.CompressionOptions{
		Enabled:       true,
		MinPageValues: 1,
		Timestamp:     "delta-of-delta",
		Float:         "xor",
		OmitWriteSeq:  true,
	})
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	got, err := unmarshalCompressedValueBlock(payload, Query{Start: 0, End: 1 << 62})
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if len(got.Samples) != 64 || got.Samples[10].Value.Float64 != 13 {
		t.Fatalf("samples wrong: %#v", got.Samples[10])
	}
}

func TestFloatIntSamplePath(t *testing.T) {
	samples := makeSamples(model.FieldFloat64, 20, func(i int) model.FieldValue {
		return model.Float64Value(float64(i * i))
	})
	intSamples, ok := floatSamplesAsIntSamples(samples)
	if !ok {
		t.Fatal("expected integer floats")
	}
	payload := appendDeltaIntValues(nil, intSamples)
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for i := range samples {
		timestamps[i] = samples[i].Timestamp
		writeSeqs[i] = samples[i].WriteSeq
	}
	got, err := readFloatIntSampleValues(
		newBlockReader(payload),
		compressionDelta,
		timestamps,
		writeSeqs,
		Query{Start: 4, End: 6},
	)
	if err != nil || len(got) != 3 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if got[0].Value.Float64 != 16 || got[2].Value.Float64 != 36 {
		t.Fatalf("values=%v %v", got[0].Value.Float64, got[2].Value.Float64)
	}
	// rle integer floats
	mono := makeSamples(model.FieldFloat64, 16, func(i int) model.FieldValue {
		return model.Float64Value(float64(i))
	})
	intMono, ok := floatSamplesAsIntSamples(mono)
	if !ok {
		t.Fatal("mono int convert failed")
	}
	rlePayload := appendDeltaRLEIntValues(nil, intMono)
	values, err := readFloatIntValues(newBlockReader(rlePayload), compressionRLE, len(mono))
	if err != nil {
		t.Fatalf("rle decode error = %v", err)
	}
	for i := range mono {
		if values[i].Float64 != mono[i].Value.Float64 {
			t.Fatalf("rle value[%d]=%v", i, values[i].Float64)
		}
	}
	if _, err := readFloatIntValues(newBlockReader(nil), compressionXOR, 1); err == nil {
		t.Fatal("wrong float-int codec error = nil")
	}
	if _, err := readFloatIntSampleValues(newBlockReader(nil), compressionXOR, []int64{1}, []uint64{1}, Query{}); err == nil {
		t.Fatal("sample wrong codec error = nil")
	}
	if samplesOut, err := readFloatConstStepSampleValues(newBlockReader(nil), compressionXOR, []int64{1}, []uint64{1}, Query{}); err == nil || samplesOut != nil {
		// wrong codec should error
		if err == nil {
			t.Fatal("const-step sample wrong codec error = nil")
		}
	}
	// inf reject
	inf := makeSamples(model.FieldFloat64, 2, func(int) model.FieldValue {
		return model.Float64Value(math.Inf(1))
	})
	if _, ok := floatSamplesAsIntSamples(inf); ok {
		t.Fatal("inf accepted")
	}
}
