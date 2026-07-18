package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

// 字典 payload 格式（POC 最终形态）：
//   dict_count uvarint
//   dict strings...
//   ordinal_mode byte:
//     0 = plain uvarint ordinals
//     1 = delta-RLE ordinals（first uvarint + (run, zigzag_delta)...）
//     2 = all_same（全部 ordinal=0，无后续字节；要求 dict_count>=1）

const (
	stringOrdinalPlain byte = 0
	stringOrdinalRLE   byte = 1
	stringOrdinalSame  byte = 2
)

func appendDictionaryStringValues(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return appendDictionaryPayload(dst, nil, nil, stringOrdinalPlain)
	}
	ids := make(map[string]int, 8)
	dict := make([]string, 0, 8)
	ordinals := make([]int, 0, len(samples))
	for _, sample := range samples {
		id, ok := ids[sample.Value.String]
		if !ok {
			id = len(dict)
			ids[sample.Value.String] = id
			dict = append(dict, sample.Value.String)
		}
		ordinals = append(ordinals, id)
	}
	if len(dict) == 1 {
		return appendDictionaryPayload(dst, dict, nil, stringOrdinalSame)
	}
	plainOrds := appendPlainOrdinals(make([]byte, 0, len(ordinals)*2), ordinals)
	rleOrds := appendRLEOrdinals(make([]byte, 0, 16+len(ordinals)), ordinals)
	if len(rleOrds) < len(plainOrds) {
		return appendDictionaryPayload(dst, dict, rleOrds, stringOrdinalRLE)
	}
	return appendDictionaryPayload(dst, dict, plainOrds, stringOrdinalPlain)
}

func appendDictionaryPayload(dst []byte, dict []string, ordinalPayload []byte, mode byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(dict)))
	for _, value := range dict {
		dst = codec.AppendString(dst, value)
	}
	dst = append(dst, mode)
	return append(dst, ordinalPayload...)
}

func appendPlainOrdinals(dst []byte, ordinals []int) []byte {
	for _, ordinal := range ordinals {
		dst = binary.AppendUvarint(dst, uint64(ordinal))
	}
	return dst
}

func appendRLEOrdinals(dst []byte, ordinals []int) []byte {
	if len(ordinals) == 0 {
		return dst
	}
	prev := ordinals[0]
	dst = binary.AppendUvarint(dst, uint64(prev))
	if len(ordinals) == 1 {
		return dst
	}
	index := 1
	for index < len(ordinals) {
		next := ordinals[index]
		delta := int64(next - prev)
		run := 1
		prev = next
		index++
		for index < len(ordinals) {
			candidate := ordinals[index]
			if int64(candidate-prev) != delta {
				break
			}
			prev = candidate
			run++
			index++
		}
		dst = binary.AppendUvarint(dst, uint64(run))
		dst = binary.AppendUvarint(dst, zigZag64(delta))
	}
	return dst
}

func readStringDictionary(reader *blockReader) ([]string, byte, error) {
	dictCount, err := reader.intCount("string dictionary count")
	if err != nil {
		return nil, 0, err
	}
	dict := make([]string, dictCount)
	for index := range dict {
		dict[index], err = reader.string("string dictionary value")
		if err != nil {
			return nil, 0, err
		}
	}
	if len(reader.rest) == 0 {
		// 兼容：旧 payload 无 mode 字节时按 plain ordinals 处理。
		return dict, stringOrdinalPlain, nil
	}
	mode, err := reader.byte("string ordinal mode")
	if err != nil {
		return nil, 0, err
	}
	return dict, mode, nil
}

func readStringDictionaryOrdinals(
	reader *blockReader,
	dict []string,
	mode byte,
	count int,
) ([]model.FieldValue, error) {
	ordinals, err := readOrdinals(reader, mode, count)
	if err != nil {
		return nil, err
	}
	values := make([]model.FieldValue, count)
	for index, ordinal := range ordinals {
		if ordinal < 0 || ordinal >= len(dict) {
			return nil, fmt.Errorf("string dictionary ordinal %d out of range", ordinal)
		}
		values[index] = model.StringValue(dict[ordinal])
	}
	return values, nil
}

func readOrdinals(reader *blockReader, mode byte, count int) ([]int, error) {
	switch mode {
	case stringOrdinalSame:
		if count == 0 {
			return nil, nil
		}
		ordinals := make([]int, count)
		return ordinals, nil
	case stringOrdinalPlain:
		return readPlainOrdinals(reader, count)
	case stringOrdinalRLE:
		return readRLEOrdinals(reader, count)
	default:
		return nil, fmt.Errorf("unknown string ordinal mode %d", mode)
	}
}

func readPlainOrdinals(reader *blockReader, count int) ([]int, error) {
	ordinals := make([]int, count)
	for index := range ordinals {
		value, err := reader.intCount("string dictionary ordinal")
		if err != nil {
			return nil, err
		}
		ordinals[index] = value
	}
	return ordinals, nil
}

func readRLEOrdinals(reader *blockReader, count int) ([]int, error) {
	if count == 0 {
		return nil, nil
	}
	first, err := reader.intCount("first string ordinal")
	if err != nil {
		return nil, err
	}
	ordinals := make([]int, count)
	ordinals[0] = first
	filled := 1
	prev := first
	for filled < count {
		run64, err := reader.uvarint("string ordinal rle run")
		if err != nil {
			return nil, err
		}
		if run64 == 0 {
			return nil, fmt.Errorf("string ordinal rle run must be > 0")
		}
		delta, err := reader.uvarint("string ordinal rle delta")
		if err != nil {
			return nil, err
		}
		step := int(unzigZag64(delta))
		for range run64 {
			if filled >= count {
				return nil, fmt.Errorf("string ordinal rle overflow")
			}
			prev += step
			ordinals[filled] = prev
			filled++
		}
	}
	return ordinals, nil
}

func readDictionaryStringSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionDictionary {
		return nil, fmt.Errorf("unknown string compression %d", codecID)
	}
	dict, mode, err := readStringDictionary(reader)
	if err != nil {
		return nil, err
	}
	ordinals, err := readOrdinals(reader, mode, len(timestamps))
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	for index, timestamp := range timestamps {
		ordinal := ordinals[index]
		if ordinal < 0 || ordinal >= len(dict) {
			return nil, fmt.Errorf("string dictionary ordinal %d out of range", ordinal)
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[index],
			Value:     model.StringValue(dict[ordinal]),
		})
	}
	return samples, nil
}
