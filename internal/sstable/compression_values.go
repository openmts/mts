package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
)

func encodeTypedValues(column model.ColumnData, opts model.CompressionOptions) (byte, []byte, error) {
	switch column.FieldType {
	case model.FieldFloat64:
		return encodeFloatValues(column.Samples, opts.Float)
	case model.FieldInt64:
		return encodeIntValues(column.Samples, opts.Int)
	case model.FieldBool:
		payload, err := appendBoolValues(nil, column.Samples)
		return compressionPlain, payload, err
	case model.FieldString:
		return encodeStringValues(column.Samples, opts.String)
	default:
		return 0, nil, fmt.Errorf("unsupported value block field type %d", column.FieldType)
	}
}

func encodeFloatValues(samples []model.VersionedSample, policy string) (byte, []byte, error) {
	plain := appendFloatValues(nil, samples)
	if compressionPolicy(policy, "xor") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendXORFloatValues(nil, samples)
	if len(candidate) < len(plain) {
		return compressionXOR, candidate, nil
	}
	return compressionPlain, plain, nil
}

func appendXORFloatValues(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	prev := math.Float64bits(samples[0].Value.Float64)
	dst = binary.LittleEndian.AppendUint64(dst, prev)
	for index := 1; index < len(samples); index++ {
		next := math.Float64bits(samples[index].Value.Float64)
		dst = binary.AppendUvarint(dst, next^prev)
		prev = next
	}
	return dst
}

func encodeIntValues(samples []model.VersionedSample, policy string) (byte, []byte, error) {
	plain := appendIntValues(nil, samples)
	if compressionPolicy(policy, "delta") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDeltaIntValues(nil, samples)
	if len(candidate) < len(plain) {
		return compressionDelta, candidate, nil
	}
	return compressionPlain, plain, nil
}

func appendDeltaIntValues(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	prev := samples[0].Value.Int64
	dst = binary.AppendVarint(dst, prev)
	for index := 1; index < len(samples); index++ {
		next := samples[index].Value.Int64
		dst = binary.AppendUvarint(dst, zigZag64(next-prev))
		prev = next
	}
	return dst
}

func encodeStringValues(samples []model.VersionedSample, policy string) (byte, []byte, error) {
	plain := appendStringValues(nil, samples)
	if compressionPolicy(policy, "dictionary") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDictionaryStringValues(nil, samples)
	if len(candidate) < len(plain) {
		return compressionDictionary, candidate, nil
	}
	return compressionPlain, plain, nil
}

func appendDictionaryStringValues(dst []byte, samples []model.VersionedSample) []byte {
	ids := make(map[string]int, len(samples))
	dict := make([]string, 0, len(samples))
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
	return appendDictionaryPayload(dst, dict, ordinals)
}

func appendDictionaryPayload(dst []byte, dict []string, ordinals []int) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(dict)))
	for _, value := range dict {
		dst = codec.AppendString(dst, value)
	}
	for _, ordinal := range ordinals {
		dst = binary.AppendUvarint(dst, uint64(ordinal))
	}
	return dst
}

func readCodecValues(
	reader *blockReader,
	fieldType model.FieldType,
	count int,
) ([]model.FieldValue, error) {
	codecID, payload, err := readCodecPayload(reader, "values")
	if err != nil {
		return nil, err
	}
	payloadReader := newBlockReader(payload)
	values, err := readCompressedValues(payloadReader, fieldType, codecID, count)
	if err != nil {
		return nil, err
	}
	return values, payloadReader.done("values")
}

func readCompressedValues(
	reader *blockReader,
	fieldType model.FieldType,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID == compressionPlain {
		return readValues(reader, fieldType, count)
	}
	switch fieldType {
	case model.FieldFloat64:
		return readXORFloatValues(reader, codecID, count)
	case model.FieldInt64:
		return readDeltaIntValues(reader, codecID, count)
	case model.FieldString:
		return readDictionaryStringValues(reader, codecID, count)
	default:
		return nil, fmt.Errorf("unsupported value compression %d for field type %d", codecID, fieldType)
	}
}

func readXORFloatValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionXOR {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	values := make([]model.FieldValue, count)
	if count == 0 {
		return values, nil
	}
	first, err := reader.fixedInt64("first float bits")
	if err != nil {
		return nil, err
	}
	prev := uint64(first)
	values[0] = model.Float64Value(math.Float64frombits(prev))
	for index := 1; index < count; index++ {
		xor, err := reader.uvarint("float xor")
		if err != nil {
			return nil, err
		}
		prev ^= xor
		values[index] = model.Float64Value(math.Float64frombits(prev))
	}
	return values, nil
}

func readDeltaIntValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionDelta {
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
	for index := 1; index < count; index++ {
		delta, err := reader.uvarint("int delta")
		if err != nil {
			return nil, err
		}
		prev += unzigZag64(delta)
		values[index] = model.Int64Value(prev)
	}
	return values, nil
}

func readDictionaryStringValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionDictionary {
		return nil, fmt.Errorf("unknown string compression %d", codecID)
	}
	dict, err := readStringDictionary(reader)
	if err != nil {
		return nil, err
	}
	return readStringDictionaryOrdinals(reader, dict, count)
}

func readStringDictionary(reader *blockReader) ([]string, error) {
	dictCount, err := reader.intCount("string dictionary count")
	if err != nil {
		return nil, err
	}
	dict := make([]string, dictCount)
	for index := range dictCount {
		dict[index], err = reader.string("string dictionary value")
		if err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func readStringDictionaryOrdinals(
	reader *blockReader,
	dict []string,
	count int,
) ([]model.FieldValue, error) {
	values := make([]model.FieldValue, count)
	for index := range count {
		ordinal, err := reader.intCount("string dictionary ordinal")
		if err != nil {
			return nil, err
		}
		if ordinal >= len(dict) {
			return nil, fmt.Errorf("string dictionary ordinal %d out of range", ordinal)
		}
		values[index] = model.StringValue(dict[ordinal])
	}
	return values, nil
}
