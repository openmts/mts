package sstable

import (
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func readWindowedFloatValues(
	reader *blockReader,
	codecID byte,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	switch codecID {
	case compressionConstStep:
		return readWindowedFloatConstStep(reader, fullCount, start, timestamps, writeSeqs, query)
	case compressionDelta, compressionRLE:
		// 先解出整页 int，再转 float 并切片窗口（页内仍需前缀状态）。
		// 相对全量路径，省掉了全量 timestamps/writeSeq 分配与二次时间过滤。
		return readWindowedFloatIntCodec(reader, codecID, fullCount, start, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unsupported windowed float codec %d", codecID)
	}
}

func readWindowedFloatConstStep(
	reader *blockReader,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if fullCount == 0 || len(timestamps) == 0 {
		return nil, nil
	}
	params, err := readFloatConstStepParams(reader)
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, len(timestamps))
	for offset, timestamp := range timestamps {
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		pageIndex := start + offset
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[offset],
			Value:     model.Float64Value(floatConstStepValue(params, pageIndex)),
		})
	}
	return samples, nil
}

func readWindowedFloatIntCodec(
	reader *blockReader,
	codecID byte,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	intSamples, err := readWindowedIntValues(reader, codecID, fullCount, start, timestamps, writeSeqs, query)
	if err != nil {
		return nil, err
	}
	for index := range intSamples {
		intSamples[index].Value = model.Float64Value(float64(intSamples[index].Value.Int64))
	}
	return intSamples, nil
}

func readWindowedIntValues(
	reader *blockReader,
	codecID byte,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	switch codecID {
	case compressionRLE:
		return readWindowedDeltaRLEInt(reader, fullCount, start, timestamps, writeSeqs, query)
	case compressionDelta:
		return readWindowedDeltaInt(reader, fullCount, start, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unsupported windowed int codec %d", codecID)
	}
}

func readWindowedDeltaInt(
	reader *blockReader,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if fullCount == 0 {
		return nil, nil
	}
	prev, err := reader.varint("first int value")
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, len(timestamps))
	end := start + len(timestamps)
	if start == 0 {
		samples = appendCompressedIntSample(samples, timestamps[0], writeSeqs[0], prev, query)
	}
	for index := 1; index < fullCount; index++ {
		delta, err := reader.uvarint("int delta")
		if err != nil {
			return nil, err
		}
		prev += unzigZag64(delta)
		if index < start || index >= end {
			continue
		}
		offset := index - start
		samples = appendCompressedIntSample(samples, timestamps[offset], writeSeqs[offset], prev, query)
	}
	return samples, nil
}

func readWindowedDeltaRLEInt(
	reader *blockReader,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if fullCount == 0 {
		return nil, nil
	}
	prev, err := reader.varint("first int value")
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, len(timestamps))
	end := start + len(timestamps)
	if start == 0 {
		samples = appendCompressedIntSample(samples, timestamps[0], writeSeqs[0], prev, query)
	}
	filled := 1
	for filled < fullCount {
		run64, err := reader.uvarint("int rle run")
		if err != nil {
			return nil, err
		}
		if run64 == 0 {
			return nil, fmt.Errorf("int rle run must be > 0")
		}
		delta, err := reader.uvarint("int rle delta")
		if err != nil {
			return nil, err
		}
		step := unzigZag64(delta)
		for range run64 {
			if filled >= fullCount {
				return nil, fmt.Errorf("int rle overflow: filled %d count %d", filled, fullCount)
			}
			prev += step
			if filled >= start && filled < end {
				offset := filled - start
				samples = appendCompressedIntSample(samples, timestamps[offset], writeSeqs[offset], prev, query)
			}
			filled++
		}
	}
	return samples, nil
}
