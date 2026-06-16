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
	valueEncodingV3     byte = 3
	valueEncodingV4     byte = 4
	valueEncodingV5     byte = 5

	timeRefModeAligned byte = 0
	timeRefModeIndexed byte = 1
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

func marshalValueBlockWithTimestamps(
	dst []byte,
	column model.ColumnData,
	rowTimestamps []int64,
) ([]byte, error) {
	dst = append(dst, valueEncodingV3)
	dst = binary.AppendUvarint(dst, uint64(column.FieldID))
	dst = append(dst, byte(column.FieldType))
	dst = binary.AppendUvarint(dst, uint64(len(column.Samples)))
	if len(column.Samples) == 0 {
		return append(dst, timeRefModeAligned), nil
	}
	mode, ordinals, err := encodeTimeRefs(column.Samples, rowTimestamps)
	if err != nil {
		return nil, err
	}
	dst = append(dst, mode)
	if mode == timeRefModeIndexed {
		dst = appendOrdinals(dst, ordinals)
	}
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

func unmarshalValueBlockWithTimestamps(
	payload []byte,
	rowTimestamps []int64,
	query Query,
) (valueBlock, error) {
	if len(payload) == 0 {
		return valueBlock{}, fmt.Errorf("decode sstable value encoding: missing byte")
	}
	if payload[0] == valueEncodingV2 {
		block, err := unmarshalValueBlock(payload)
		if err != nil {
			return valueBlock{}, err
		}
		return filterValueBlock(block, query), nil
	}
	if payload[0] == valueEncodingV4 {
		return valueBlock{}, fmt.Errorf("value block v4 index must be decoded by readValueColumn")
	}
	if payload[0] == valueEncodingV5 {
		return unmarshalCompressedValueBlock(payload, query)
	}
	reader := newBlockReader(payload)
	header, err := readValueHeaderV3(reader)
	if err != nil {
		return valueBlock{}, err
	}
	timestamps, err := readTimeRefs(reader, header.count, rowTimestamps)
	if err != nil {
		return valueBlock{}, err
	}
	writeSeqs, err := readWriteSeqs(reader, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	samples, err := readSamples(reader, header.fieldType, timestamps, writeSeqs, query)
	if err != nil {
		return valueBlock{}, err
	}
	if err := reader.done("value block"); err != nil {
		return valueBlock{}, err
	}
	return valueBlock{
		Encoding:  "binary-v3",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}, nil
}

func marshalValuePageIndex(dst []byte, index valuePageIndex) ([]byte, error) {
	dst = append(dst, valueEncodingV4)
	dst = binary.AppendUvarint(dst, uint64(index.FieldID))
	dst = append(dst, byte(index.FieldType))
	dst = binary.AppendUvarint(dst, uint64(index.Count))
	dst = binary.AppendUvarint(dst, uint64(len(index.Pages)))
	for _, page := range index.Pages {
		dst = binary.AppendVarint(dst, page.MinTime)
		dst = binary.AppendVarint(dst, page.MaxTime)
		var err error
		dst, err = appendBlockRef(dst, page.Ref)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func unmarshalValuePageIndex(payload []byte) (valuePageIndex, error) {
	reader := newBlockReader(payload)
	encoding, err := reader.byte("value page index encoding")
	if err != nil {
		return valuePageIndex{}, err
	}
	if encoding != valueEncodingV4 {
		return valuePageIndex{}, fmt.Errorf("unknown value page index encoding %d", encoding)
	}
	fieldID64, err := reader.uvarint("value page index field id")
	if err != nil {
		return valuePageIndex{}, err
	}
	fieldID, err := uint32Value("value page index field id", fieldID64)
	if err != nil {
		return valuePageIndex{}, err
	}
	fieldType, err := reader.byte("value page index field type")
	if err != nil {
		return valuePageIndex{}, err
	}
	count, err := reader.intCount("value page index sample count")
	if err != nil {
		return valuePageIndex{}, err
	}
	pageCount, err := reader.intCount("value page index page count")
	if err != nil {
		return valuePageIndex{}, err
	}
	pages := make([]valuePageRef, 0, pageCount)
	for range pageCount {
		page, err := readValuePageRef(reader)
		if err != nil {
			return valuePageIndex{}, err
		}
		pages = append(pages, page)
	}
	if err := reader.done("value page index"); err != nil {
		return valuePageIndex{}, err
	}
	return valuePageIndex{
		FieldID:   fieldID,
		FieldType: model.FieldType(fieldType),
		Count:     count,
		Pages:     pages,
	}, nil
}

func readValuePageRef(reader *blockReader) (valuePageRef, error) {
	minTime, err := reader.varint("value page min time")
	if err != nil {
		return valuePageRef{}, err
	}
	maxTime, err := reader.varint("value page max time")
	if err != nil {
		return valuePageRef{}, err
	}
	ref, err := readBlockRef(reader)
	return valuePageRef{MinTime: minTime, MaxTime: maxTime, Ref: ref}, err
}

func uint32Value(name string, value uint64) (uint32, error) {
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("%s overflows uint32", name)
	}
	return uint32(value), nil
}

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

func readValueHeaderV3(reader *blockReader) (valueHeader, error) {
	encoding, err := reader.byte("value encoding")
	if err != nil {
		return valueHeader{}, err
	}
	if encoding != valueEncodingV3 {
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

func readTimeRefs(reader *blockReader, count int, rowTimestamps []int64) ([]int64, error) {
	mode, err := reader.byte("time ref mode")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return []int64{}, nil
	}
	if len(rowTimestamps) == 0 {
		return nil, fmt.Errorf("row timestamps are required for value block v3")
	}
	switch mode {
	case timeRefModeAligned:
		return readAlignedTimeRefs(count, rowTimestamps)
	case timeRefModeIndexed:
		return readIndexedTimeRefs(reader, count, rowTimestamps)
	default:
		return nil, fmt.Errorf("unknown time ref mode %d", mode)
	}
}

func readAlignedTimeRefs(count int, rowTimestamps []int64) ([]int64, error) {
	if count != len(rowTimestamps) {
		return nil, fmt.Errorf("aligned value count %d does not match row timestamp count %d", count, len(rowTimestamps))
	}
	return rowTimestamps, nil
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
		Encoding:  "binary-v2",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}
}
