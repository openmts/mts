package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

func encodeTimeRefs(
	samples []model.VersionedSample,
	rowTimestamps []int64,
) (byte, []int, error) {
	if len(samples) == 0 {
		return timeRefModeAligned, nil, nil
	}
	if len(rowTimestamps) == 0 {
		return 0, nil, fmt.Errorf("row timestamps are empty")
	}
	if samplesAligned(samples, rowTimestamps) {
		return timeRefModeAligned, nil, nil
	}
	ordinals := make([]int, 0, len(samples))
	rowIndex := 0
	for _, sample := range samples {
		for rowIndex < len(rowTimestamps) && rowTimestamps[rowIndex] < sample.Timestamp {
			rowIndex++
		}
		if rowIndex == len(rowTimestamps) || rowTimestamps[rowIndex] != sample.Timestamp {
			return 0, nil, fmt.Errorf("sample timestamp %d is missing from row time block", sample.Timestamp)
		}
		ordinals = append(ordinals, rowIndex)
		rowIndex++
	}
	return timeRefModeIndexed, ordinals, nil
}

func samplesAligned(samples []model.VersionedSample, rowTimestamps []int64) bool {
	if len(samples) != len(rowTimestamps) {
		return false
	}
	for index, sample := range samples {
		if sample.Timestamp != rowTimestamps[index] {
			return false
		}
	}
	return true
}

func appendOrdinals(dst []byte, ordinals []int) []byte {
	if len(ordinals) == 0 {
		return dst
	}
	dst = binary.AppendUvarint(dst, uint64(ordinals[0]))
	for index := 1; index < len(ordinals); index++ {
		dst = binary.AppendUvarint(dst, uint64(ordinals[index]-ordinals[index-1]))
	}
	return dst
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
	if encoding != valueEncodingPagePlain {
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

func readIndexedTimeRefs(reader *blockReader, count int, rowTimestamps []int64) ([]int64, error) {
	timestamps := make([]int64, count)
	var ordinal int
	for index := range count {
		delta, err := reader.intCount("time ordinal")
		if err != nil {
			return nil, err
		}
		if index == 0 {
			ordinal = delta
		} else {
			if delta == 0 {
				return nil, fmt.Errorf("time ordinal delta must be positive")
			}
			ordinal += delta
		}
		if ordinal >= len(rowTimestamps) {
			return nil, fmt.Errorf("time ordinal %d out of range", ordinal)
		}
		timestamps[index] = rowTimestamps[ordinal]
	}
	return timestamps, nil
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

func readSamples(
	reader *blockReader,
	fieldType model.FieldType,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	switch fieldType {
	case model.FieldFloat64:
		return readFloatSamples(reader, timestamps, writeSeqs, query)
	case model.FieldInt64:
		return readIntSamples(reader, timestamps, writeSeqs, query)
	case model.FieldBool:
		return readBoolSamples(reader, timestamps, writeSeqs, query)
	case model.FieldString:
		return readStringSamples(reader, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unsupported value block field type %d", fieldType)
	}
}

func readFloatSamples(
	reader *blockReader,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, len(timestamps))
	for index, timestamp := range timestamps {
		value, err := reader.float64()
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[index],
			Value:     model.Float64Value(value),
		})
	}
	return samples, nil
}

func readIntSamples(
	reader *blockReader,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, len(timestamps))
	for index, timestamp := range timestamps {
		value, err := reader.varint("int value")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[index],
			Value:     model.Int64Value(value),
		})
	}
	return samples, nil
}

func readBoolSamples(
	reader *blockReader,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	bools, rest, err := codec.ReadBoolBits(reader.rest, len(timestamps))
	if err != nil {
		return nil, fmt.Errorf("read bool values: %w", err)
	}
	reader.rest = rest
	samples := make([]model.VersionedSample, 0, len(timestamps))
	for index, timestamp := range timestamps {
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[index],
			Value:     model.BoolValue(bools[index]),
		})
	}
	return samples, nil
}

func readStringSamples(
	reader *blockReader,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, len(timestamps))
	for index, timestamp := range timestamps {
		value, err := reader.string("string value")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeqs[index],
			Value:     model.StringValue(value),
		})
	}
	return samples, nil
}

func filterValueBlock(block valueBlock, query Query) valueBlock {
	if query.End < query.Start {
		block.Samples = []model.VersionedSample{}
		return block
	}
	samples := make([]model.VersionedSample, 0, len(block.Samples))
	for _, sample := range block.Samples {
		if sample.Timestamp < query.Start || sample.Timestamp > query.End {
			continue
		}
		samples = append(samples, sample)
	}
	block.Samples = samples
	return block
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
		Encoding:  "binary-values",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}
}
