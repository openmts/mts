package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
)

const (
	timeEncodingDeltaV2 byte = 1
	valueEncodingV2     byte = 2
)

func marshalTimeBlock(dst []byte, timestamps []int64) []byte {
	dst = append(dst, timeEncodingDeltaV2)
	dst = binary.AppendUvarint(dst, uint64(len(timestamps)))
	if len(timestamps) == 0 {
		return dst
	}
	dst = binary.LittleEndian.AppendUint64(dst, uint64(timestamps[0]))
	for index := 1; index < len(timestamps); index++ {
		dst = binary.AppendVarint(dst, timestamps[index]-timestamps[index-1])
	}
	return dst
}

func unmarshalTimeBlock(payload []byte) ([]int64, error) {
	reader := newBlockReader(payload)
	encoding, err := reader.byte("time encoding")
	if err != nil {
		return nil, err
	}
	if encoding != timeEncodingDeltaV2 {
		return nil, fmt.Errorf("unknown time encoding %d", encoding)
	}
	count, err := reader.intCount("time count")
	if err != nil {
		return nil, err
	}
	timestamps, err := readTimestamps(reader, count)
	if err != nil {
		return nil, err
	}
	return timestamps, reader.done("time block")
}

func marshalValueBlock(dst []byte, column model.ColumnData) ([]byte, error) {
	dst = append(dst, valueEncodingV2)
	dst = binary.AppendUvarint(dst, uint64(column.FieldID))
	dst = append(dst, byte(column.FieldType))
	dst = binary.AppendUvarint(dst, uint64(len(column.Samples)))
	if len(column.Samples) == 0 {
		return dst, nil
	}
	dst = appendSampleTimes(dst, column.Samples)
	dst = appendSampleWriteSeqs(dst, column.Samples)
	return appendSampleValues(dst, column)
}

func unmarshalValueBlock(payload []byte) (valueBlock, error) {
	reader := newBlockReader(payload)
	header, err := readValueHeader(reader)
	if err != nil {
		return valueBlock{}, err
	}
	timestamps, err := readTimestamps(reader, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	writeSeqs, err := readWriteSeqs(reader, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	values, err := readValues(reader, header.fieldType, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	if err := reader.done("value block"); err != nil {
		return valueBlock{}, err
	}
	return buildValueBlock(header, timestamps, writeSeqs, values), nil
}

func appendSampleTimes(dst []byte, samples []model.VersionedSample) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, uint64(samples[0].Timestamp))
	for index := 1; index < len(samples); index++ {
		dst = binary.AppendVarint(dst, samples[index].Timestamp-samples[index-1].Timestamp)
	}
	return dst
}

func appendSampleWriteSeqs(dst []byte, samples []model.VersionedSample) []byte {
	for _, sample := range samples {
		dst = binary.AppendUvarint(dst, sample.WriteSeq)
	}
	return dst
}

func appendSampleValues(dst []byte, column model.ColumnData) ([]byte, error) {
	switch column.FieldType {
	case model.FieldFloat64:
		return appendFloatValues(dst, column.Samples), nil
	case model.FieldInt64:
		return appendIntValues(dst, column.Samples), nil
	case model.FieldBool:
		return appendBoolValues(dst, column.Samples)
	case model.FieldString:
		return appendStringValues(dst, column.Samples), nil
	default:
		return nil, fmt.Errorf("unsupported value block field type %d", column.FieldType)
	}
}

func appendFloatValues(dst []byte, samples []model.VersionedSample) []byte {
	for _, sample := range samples {
		dst = binary.LittleEndian.AppendUint64(dst, math.Float64bits(sample.Value.Float64))
	}
	return dst
}

func appendIntValues(dst []byte, samples []model.VersionedSample) []byte {
	for _, sample := range samples {
		dst = binary.AppendVarint(dst, sample.Value.Int64)
	}
	return dst
}

func appendBoolValues(dst []byte, samples []model.VersionedSample) ([]byte, error) {
	values := make([]bool, len(samples))
	for index, sample := range samples {
		if sample.Value.Type != model.FieldBool {
			return nil, fmt.Errorf("bool sample has value type %d", sample.Value.Type)
		}
		values[index] = sample.Value.Bool
	}
	return codec.AppendBoolBits(dst, values), nil
}

func appendStringValues(dst []byte, samples []model.VersionedSample) []byte {
	for _, sample := range samples {
		dst = codec.AppendString(dst, sample.Value.String)
	}
	return dst
}

type valueHeader struct {
	fieldID   uint32
	fieldType model.FieldType
	count     int
}

func readValueHeader(reader *blockReader) (valueHeader, error) {
	encoding, err := reader.byte("value encoding")
	if err != nil {
		return valueHeader{}, err
	}
	if encoding != valueEncodingV2 {
		return valueHeader{}, fmt.Errorf("unknown value encoding %d", encoding)
	}
	fieldID, err := reader.uint32("field id")
	if err != nil {
		return valueHeader{}, err
	}
	fieldType, err := reader.byte("field type")
	if err != nil {
		return valueHeader{}, err
	}
	count, err := reader.intCount("sample count")
	return valueHeader{fieldID: fieldID, fieldType: model.FieldType(fieldType), count: count}, err
}

func readTimestamps(reader *blockReader, count int) ([]int64, error) {
	if count == 0 {
		return nil, nil
	}
	first, err := reader.fixedInt64("first timestamp")
	if err != nil {
		return nil, err
	}
	timestamps := make([]int64, count)
	timestamps[0] = first
	for index := 1; index < count; index++ {
		delta, err := reader.varint("timestamp delta")
		if err != nil {
			return nil, err
		}
		timestamps[index] = timestamps[index-1] + delta
	}
	return timestamps, nil
}

func readWriteSeqs(reader *blockReader, count int) ([]uint64, error) {
	writeSeqs := make([]uint64, count)
	for index := range count {
		value, err := reader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		writeSeqs[index] = value
	}
	return writeSeqs, nil
}

func readValues(reader *blockReader, fieldType model.FieldType, count int) ([]model.FieldValue, error) {
	switch fieldType {
	case model.FieldFloat64:
		return readFloatValues(reader, count)
	case model.FieldInt64:
		return readIntValues(reader, count)
	case model.FieldBool:
		return readBoolValues(reader, count)
	case model.FieldString:
		return readStringValues(reader, count)
	default:
		return nil, fmt.Errorf("unsupported value block field type %d", fieldType)
	}
}

func readFloatValues(reader *blockReader, count int) ([]model.FieldValue, error) {
	values := make([]model.FieldValue, count)
	for index := range count {
		value, err := reader.float64()
		if err != nil {
			return nil, err
		}
		values[index] = model.Float64Value(value)
	}
	return values, nil
}

func readIntValues(reader *blockReader, count int) ([]model.FieldValue, error) {
	values := make([]model.FieldValue, count)
	for index := range count {
		value, err := reader.varint("int value")
		if err != nil {
			return nil, err
		}
		values[index] = model.Int64Value(value)
	}
	return values, nil
}

func readBoolValues(reader *blockReader, count int) ([]model.FieldValue, error) {
	bools, rest, err := codec.ReadBoolBits(reader.rest, count)
	if err != nil {
		return nil, fmt.Errorf("read bool values: %w", err)
	}
	reader.rest = rest
	values := make([]model.FieldValue, count)
	for index, value := range bools {
		values[index] = model.BoolValue(value)
	}
	return values, nil
}

func readStringValues(reader *blockReader, count int) ([]model.FieldValue, error) {
	values := make([]model.FieldValue, count)
	for index := range count {
		value, err := reader.string("string value")
		if err != nil {
			return nil, err
		}
		values[index] = model.StringValue(value)
	}
	return values, nil
}

func buildValueBlock(
	header valueHeader,
	timestamps []int64,
	writeSeqs []uint64,
	values []model.FieldValue,
) valueBlock {
	samples := make([]model.VersionedSample, header.count)
	for index := range header.count {
		samples[index] = model.VersionedSample{
			Timestamp: timestamps[index],
			WriteSeq:  writeSeqs[index],
			Value:     values[index],
		}
	}
	return valueBlock{
		Encoding:  "binary-v2",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}
}
