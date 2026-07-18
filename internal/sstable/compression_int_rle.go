package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

// Int delta-RLE 载荷：first varint + 重复 (run_len uvarint, zigzag_delta uvarint)。
// 使用 compressionRLE codec id。

func appendDeltaRLEIntValues(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	prev := samples[0].Value.Int64
	dst = binary.AppendVarint(dst, prev)
	if len(samples) == 1 {
		return dst
	}
	index := 1
	for index < len(samples) {
		delta := samples[index].Value.Int64 - prev
		run := 1
		prev = samples[index].Value.Int64
		index++
		for index < len(samples) {
			next := samples[index].Value.Int64
			if next-prev != delta {
				break
			}
			prev = next
			run++
			index++
		}
		dst = binary.AppendUvarint(dst, uint64(run))
		dst = binary.AppendUvarint(dst, zigZag64(delta))
	}
	return dst
}

func readDeltaRLEIntValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionRLE {
		return nil, fmt.Errorf("unknown int compression %d", codecID)
	}
	values := make([]model.FieldValue, count)
	if count == 0 {
		return values, nil
	}
	prev, err := reader.varint("first int value")
	if err != nil {
		return nil, err
	}
	values[0] = model.Int64Value(prev)
	filled := 1
	for filled < count {
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
			if filled >= count {
				return nil, fmt.Errorf("int rle overflow: filled %d count %d", filled, count)
			}
			prev += step
			values[filled] = model.Int64Value(prev)
			filled++
		}
	}
	return values, nil
}

func readDeltaRLEIntSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionRLE {
		return nil, fmt.Errorf("unknown int compression %d", codecID)
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	if len(timestamps) == 0 {
		return samples, nil
	}
	prev, err := reader.varint("first int value")
	if err != nil {
		return nil, err
	}
	samples = appendCompressedIntSample(samples, timestamps[0], writeSeqs[0], prev, query)
	filled := 1
	for filled < len(timestamps) {
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
			if filled >= len(timestamps) {
				return nil, fmt.Errorf("int rle overflow: filled %d count %d", filled, len(timestamps))
			}
			prev += step
			samples = appendCompressedIntSample(samples, timestamps[filled], writeSeqs[filled], prev, query)
			filled++
		}
	}
	return samples, nil
}
