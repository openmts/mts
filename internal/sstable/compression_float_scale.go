package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/model"
)

// float 专用载荷：
// 1) compressionConstStep：base(float64 LE) + step(float64 LE)，值 = base + i*step
// 2) compressionDelta / compressionRLE：整数值 float 走 int 编码，解码时转回 float64

func appendFloatConstStepValues(dst []byte, base, step float64) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, math.Float64bits(base))
	return binary.LittleEndian.AppendUint64(dst, math.Float64bits(step))
}

func detectFloatConstStep(samples []model.VersionedSample) (base, step float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, false
	}
	base = samples[0].Value.Float64
	step = samples[1].Value.Float64 - base
	// 用 base+i*step 精确匹配（与 scale 的 index*k 生成方式一致）。
	for index := 1; index < len(samples); index++ {
		if samples[index].Value.Float64 != base+float64(index)*step {
			return 0, 0, false
		}
	}
	return base, step, true
}

func floatSamplesAsIntSamples(samples []model.VersionedSample) ([]model.VersionedSample, bool) {
	if len(samples) == 0 {
		return nil, true
	}
	out := make([]model.VersionedSample, len(samples))
	for index, sample := range samples {
		value := sample.Value.Float64
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
		if value != math.Trunc(value) {
			return nil, false
		}
		if value < float64(math.MinInt64) || value > float64(math.MaxInt64) {
			return nil, false
		}
		out[index] = model.VersionedSample{
			Timestamp: sample.Timestamp,
			WriteSeq:  sample.WriteSeq,
			Value:     model.Int64Value(int64(value)),
		}
	}
	return out, true
}

func readFloatConstStepValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionConstStep {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	if count == 0 {
		return nil, nil
	}
	baseBits, err := reader.fixedInt64("float const-step base")
	if err != nil {
		return nil, err
	}
	stepBits, err := reader.fixedInt64("float const-step step")
	if err != nil {
		return nil, err
	}
	base := math.Float64frombits(uint64(baseBits))
	step := math.Float64frombits(uint64(stepBits))
	values := make([]model.FieldValue, count)
	for index := range values {
		values[index] = model.Float64Value(base + float64(index)*step)
	}
	return values, nil
}

func readFloatConstStepSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionConstStep {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	if len(timestamps) == 0 {
		return nil, nil
	}
	baseBits, err := reader.fixedInt64("float const-step base")
	if err != nil {
		return nil, err
	}
	stepBits, err := reader.fixedInt64("float const-step step")
	if err != nil {
		return nil, err
	}
	base := math.Float64frombits(uint64(baseBits))
	step := math.Float64frombits(uint64(stepBits))
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	for index, timestamp := range timestamps {
		if timestamp >= query.Start && timestamp <= query.End {
			samples = append(samples, model.VersionedSample{
				Timestamp: timestamp,
				WriteSeq:  writeSeqs[index],
				Value:     model.Float64Value(base + float64(index)*step),
			})
		}
	}
	return samples, nil
}

func readFloatIntValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	var (
		intValues []model.FieldValue
		err       error
	)
	switch codecID {
	case compressionDelta:
		intValues, err = readDeltaIntValues(reader, codecID, count)
	case compressionRLE:
		intValues, err = readDeltaRLEIntValues(reader, codecID, count)
	default:
		return nil, fmt.Errorf("unknown float-int compression %d", codecID)
	}
	if err != nil {
		return nil, err
	}
	values := make([]model.FieldValue, count)
	for index, value := range intValues {
		values[index] = model.Float64Value(float64(value.Int64))
	}
	return values, nil
}

func readFloatIntSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	var (
		intSamples []model.VersionedSample
		err        error
	)
	switch codecID {
	case compressionDelta:
		intSamples, err = readDeltaIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case compressionRLE:
		intSamples, err = readDeltaRLEIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unknown float-int compression %d", codecID)
	}
	if err != nil {
		return nil, err
	}
	for index := range intSamples {
		intSamples[index].Value = model.Float64Value(float64(intSamples[index].Value.Int64))
	}
	return intSamples, nil
}
