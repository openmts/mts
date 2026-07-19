package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/model"
)

// float 专用载荷：
// compressionConstStep：
//   kind=0 AP：base(float64 LE)+step(float64 LE)，值 = base + float64(i)*step
//   kind=1 IndexScale：start(int64 LE)+scale(float64 LE)，值 = float64(start+i)*scale
// compressionDelta / compressionRLE：整数值 float 走 int 编码，解码时转回 float64

const (
	floatConstStepKindAP         byte = 0
	floatConstStepKindIndexScale byte = 1
)

func appendFloatConstStepAP(dst []byte, base, step float64) []byte {
	dst = append(dst, floatConstStepKindAP)
	dst = binary.LittleEndian.AppendUint64(dst, math.Float64bits(base))
	return binary.LittleEndian.AppendUint64(dst, math.Float64bits(step))
}

func appendFloatConstStepIndexScale(dst []byte, start int64, scale float64) []byte {
	dst = append(dst, floatConstStepKindIndexScale)
	dst = binary.LittleEndian.AppendUint64(dst, uint64(start))
	return binary.LittleEndian.AppendUint64(dst, math.Float64bits(scale))
}

// appendFloatConstStepValues 兼容旧测试入口，按 AP 编码。
func appendFloatConstStepValues(dst []byte, base, step float64) []byte {
	return appendFloatConstStepAP(dst, base, step)
}

func detectFloatConstStep(samples []model.VersionedSample) (base, step float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, false
	}
	base = samples[0].Value.Float64
	step = samples[1].Value.Float64 - base
	for index := 1; index < len(samples); index++ {
		if samples[index].Value.Float64 != base+float64(index)*step {
			return 0, 0, false
		}
	}
	return base, step, true
}

func detectFloatIndexScale(samples []model.VersionedSample) (start int64, scale float64, ok bool) {
	if len(samples) == 0 {
		return 0, 0, false
	}
	if len(samples) == 1 {
		value := samples[0].Value.Float64
		if value == 0 {
			return 0, 0, true
		}
		return 1, value, true
	}
	v0 := samples[0].Value.Float64
	v1 := samples[1].Value.Float64
	candidates := make([]int64, 0, 8)
	if v0 == 0 {
		candidates = append(candidates, 0)
	}
	if v1 != v0 {
		approx := v0 / (v1 - v0)
		base := int64(math.Round(approx))
		for _, delta := range []int64{0, -1, 1, -2, 2, -3, 3} {
			candidates = append(candidates, base+delta)
		}
	}
	// 去重。
	seen := make(map[int64]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		var candidateScale float64
		if candidate == 0 {
			if v0 != 0 {
				continue
			}
			candidateScale = v1
		} else {
			candidateScale = v0 / float64(candidate)
		}
		if matchesFloatIndexScale(samples, candidate, candidateScale) {
			return candidate, candidateScale, true
		}
	}
	return 0, 0, false
}

func matchesFloatIndexScale(samples []model.VersionedSample, start int64, scale float64) bool {
	for index, sample := range samples {
		if float64(start+int64(index))*scale != sample.Value.Float64 {
			return false
		}
	}
	return true
}

func encodeFloatConstStepPayload(samples []model.VersionedSample) ([]byte, bool) {
	if start, scale, ok := detectFloatIndexScale(samples); ok {
		return appendFloatConstStepIndexScale(make([]byte, 0, 17), start, scale), true
	}
	if base, step, ok := detectFloatConstStep(samples); ok {
		return appendFloatConstStepAP(make([]byte, 0, 17), base, step), true
	}
	return nil, false
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

type floatConstStepParams struct {
	kind  byte
	base  float64
	step  float64
	start int64
	scale float64
}

func readFloatConstStepParams(reader *blockReader) (floatConstStepParams, error) {
	// 旧载荷固定 16 字节 AP（无 kind）。新载荷 1+16 字节带 kind。
	if len(reader.rest) == 16 {
		baseBits, err := reader.fixedInt64("float const-step base")
		if err != nil {
			return floatConstStepParams{}, err
		}
		stepBits, err := reader.fixedInt64("float const-step step")
		if err != nil {
			return floatConstStepParams{}, err
		}
		return floatConstStepParams{
			kind: floatConstStepKindAP,
			base: math.Float64frombits(uint64(baseBits)),
			step: math.Float64frombits(uint64(stepBits)),
		}, nil
	}
	kind, err := reader.byte("float const-step kind")
	if err != nil {
		return floatConstStepParams{}, err
	}
	switch kind {
	case floatConstStepKindAP:
		baseBits, err := reader.fixedInt64("float const-step base")
		if err != nil {
			return floatConstStepParams{}, err
		}
		stepBits, err := reader.fixedInt64("float const-step step")
		if err != nil {
			return floatConstStepParams{}, err
		}
		return floatConstStepParams{
			kind: floatConstStepKindAP,
			base: math.Float64frombits(uint64(baseBits)),
			step: math.Float64frombits(uint64(stepBits)),
		}, nil
	case floatConstStepKindIndexScale:
		startBits, err := reader.fixedInt64("float const-step start")
		if err != nil {
			return floatConstStepParams{}, err
		}
		scaleBits, err := reader.fixedInt64("float const-step scale")
		if err != nil {
			return floatConstStepParams{}, err
		}
		return floatConstStepParams{
			kind:  floatConstStepKindIndexScale,
			start: startBits,
			scale: math.Float64frombits(uint64(scaleBits)),
		}, nil
	default:
		return floatConstStepParams{}, fmt.Errorf("unknown float const-step kind %d", kind)
	}
}

func floatConstStepValue(params floatConstStepParams, index int) float64 {
	switch params.kind {
	case floatConstStepKindIndexScale:
		return float64(params.start+int64(index)) * params.scale
	default:
		return params.base + float64(index)*params.step
	}
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
	params, err := readFloatConstStepParams(reader)
	if err != nil {
		return nil, err
	}
	values := make([]model.FieldValue, count)
	for index := range values {
		values[index] = model.Float64Value(floatConstStepValue(params, index))
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
	params, err := readFloatConstStepParams(reader)
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	for index, timestamp := range timestamps {
		if timestamp >= query.Start && timestamp <= query.End {
			samples = append(samples, model.VersionedSample{
				Timestamp: timestamp,
				WriteSeq:  writeSeqs[index],
				Value:     model.Float64Value(floatConstStepValue(params, index)),
			})
		}
	}
	return samples, nil
}
