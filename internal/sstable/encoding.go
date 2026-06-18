package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

const (
	timeEncodingDelta           byte = 1
	valueEncodingPagePlain      byte = 3
	valueEncodingPageIndex      byte = 4
	valueEncodingPageCompressed byte = 5

	timeRefModeAligned byte = 0
	timeRefModeIndexed byte = 1
)

func marshalTimeBlock(dst []byte, timestamps []int64) []byte {
	dst = append(dst, timeEncodingDelta)
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
	if encoding != timeEncodingDelta {
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

func marshalValueBlockWithTimestamps(
	dst []byte,
	column model.ColumnData,
	rowTimestamps []int64,
) ([]byte, error) {
	dst = append(dst, valueEncodingPagePlain)
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

func unmarshalValueBlockWithTimestamps(
	payload []byte,
	rowTimestamps []int64,
	query Query,
) (valueBlock, error) {
	if len(payload) == 0 {
		return valueBlock{}, fmt.Errorf("decode sstable value encoding: missing byte")
	}
	if payload[0] == valueEncodingPageIndex {
		return valueBlock{}, fmt.Errorf("value page index must be decoded by readValueColumn")
	}
	if payload[0] == valueEncodingPageCompressed {
		return unmarshalCompressedValueBlock(payload, query)
	}
	reader := newBlockReader(payload)
	header, err := readValueHeader(reader)
	if err != nil {
		return valueBlock{}, err
	}
	mode, err := reader.byte("time ref mode")
	if err != nil {
		return valueBlock{}, err
	}
	if !supportedValueFieldType(header.fieldType) {
		return valueBlock{}, fmt.Errorf("unsupported value block field type %d", header.fieldType)
	}
	if header.count == 0 {
		if err := reader.done("value block"); err != nil {
			return valueBlock{}, err
		}
		return valueBlock{
			Encoding:  "binary-page",
			FieldID:   header.fieldID,
			FieldType: header.fieldType,
		}, nil
	}
	if len(rowTimestamps) == 0 {
		return valueBlock{}, fmt.Errorf("row timestamps are required for value page")
	}
	if mode == timeRefModeAligned {
		samples, err := readAlignedSamples(reader, header.fieldType, header.count, rowTimestamps, query)
		if err != nil {
			return valueBlock{}, err
		}
		if err := reader.done("value block"); err != nil {
			return valueBlock{}, err
		}
		return valueBlock{
			Encoding:  "binary-page",
			FieldID:   header.fieldID,
			FieldType: header.fieldType,
			Samples:   samples,
		}, nil
	}
	if mode != timeRefModeIndexed {
		return valueBlock{}, fmt.Errorf("unknown time ref mode %d", mode)
	}
	samples, err := readIndexedSamples(reader, header.fieldType, header.count, rowTimestamps, query)
	if err != nil {
		return valueBlock{}, err
	}
	if err := reader.done("value block"); err != nil {
		return valueBlock{}, err
	}
	return valueBlock{
		Encoding:  "binary-page",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}, nil
}

func supportedValueFieldType(fieldType model.FieldType) bool {
	switch fieldType {
	case model.FieldFloat64, model.FieldInt64, model.FieldBool, model.FieldString:
		return true
	default:
		return false
	}
}

func readAlignedSamples(
	reader *blockReader,
	fieldType model.FieldType,
	count int,
	rowTimestamps []int64,
	query Query,
) ([]model.VersionedSample, error) {
	if count != len(rowTimestamps) {
		return nil, fmt.Errorf("aligned value count %d does not match row timestamp count %d", count, len(rowTimestamps))
	}
	samples := make([]model.VersionedSample, 0, matchingTimestampCount(rowTimestamps, query))
	for _, timestamp := range rowTimestamps {
		writeSeq, err := reader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeq,
		})
	}
	if err := fillAlignedSampleValues(reader, fieldType, rowTimestamps, query, samples); err != nil {
		return nil, err
	}
	return samples, nil
}

func readIndexedSamples(
	reader *blockReader,
	fieldType model.FieldType,
	count int,
	rowTimestamps []int64,
	query Query,
) ([]model.VersionedSample, error) {
	samples, sourceIndexes, err := readIndexedSampleTimes(reader, count, rowTimestamps, query)
	if err != nil {
		return nil, err
	}
	if err := fillIndexedSampleWriteSeqs(reader, count, samples, sourceIndexes); err != nil {
		return nil, err
	}
	if err := fillIndexedSampleValues(reader, fieldType, count, samples, sourceIndexes); err != nil {
		return nil, err
	}
	return samples, nil
}

func readIndexedSampleTimes(
	reader *blockReader,
	count int,
	rowTimestamps []int64,
	query Query,
) ([]model.VersionedSample, []int, error) {
	capacity := min(count, matchingTimestampCount(rowTimestamps, query))
	samples := make([]model.VersionedSample, 0, capacity)
	sourceIndexes := make([]int, 0, capacity)
	var ordinal int
	for index := range count {
		delta, err := reader.intCount("time ordinal")
		if err != nil {
			return nil, nil, err
		}
		if index == 0 {
			ordinal = delta
		} else {
			if delta == 0 {
				return nil, nil, fmt.Errorf("time ordinal delta must be positive")
			}
			ordinal += delta
		}
		if ordinal >= len(rowTimestamps) {
			return nil, nil, fmt.Errorf("time ordinal %d out of range", ordinal)
		}
		timestamp := rowTimestamps[ordinal]
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{Timestamp: timestamp})
		sourceIndexes = append(sourceIndexes, index)
	}
	return samples, sourceIndexes, nil
}

func fillIndexedSampleWriteSeqs(
	reader *blockReader,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	match := 0
	for index := range count {
		value, err := reader.uvarint("write seq")
		if err != nil {
			return err
		}
		if match < len(sourceIndexes) && sourceIndexes[match] == index {
			samples[match].WriteSeq = value
			match++
		}
	}
	return nil
}

func fillIndexedSampleValues(
	reader *blockReader,
	fieldType model.FieldType,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	switch fieldType {
	case model.FieldFloat64:
		return fillIndexedFloatValues(reader, count, samples, sourceIndexes)
	case model.FieldInt64:
		return fillIndexedIntValues(reader, count, samples, sourceIndexes)
	case model.FieldBool:
		return fillIndexedBoolValues(reader, count, samples, sourceIndexes)
	case model.FieldString:
		return fillIndexedStringValues(reader, count, samples, sourceIndexes)
	default:
		return fmt.Errorf("unsupported value block field type %d", fieldType)
	}
}

func fillIndexedFloatValues(
	reader *blockReader,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	match := 0
	for index := range count {
		value, err := reader.float64()
		if err != nil {
			return err
		}
		if match < len(sourceIndexes) && sourceIndexes[match] == index {
			samples[match].Value = model.Float64Value(value)
			match++
		}
	}
	return nil
}

func fillIndexedIntValues(
	reader *blockReader,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	match := 0
	for index := range count {
		value, err := reader.varint("int value")
		if err != nil {
			return err
		}
		if match < len(sourceIndexes) && sourceIndexes[match] == index {
			samples[match].Value = model.Int64Value(value)
			match++
		}
	}
	return nil
}

func fillIndexedBoolValues(
	reader *blockReader,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	byteCount := (count + 7) / 8
	if len(reader.rest) < byteCount {
		return fmt.Errorf("read bool values: read bool bits: truncated payload")
	}
	bits := reader.rest[:byteCount]
	reader.rest = reader.rest[byteCount:]
	for index, sourceIndex := range sourceIndexes {
		value := bits[sourceIndex/8]&(1<<uint(sourceIndex%8)) != 0
		samples[index].Value = model.BoolValue(value)
	}
	return nil
}

func fillIndexedStringValues(
	reader *blockReader,
	count int,
	samples []model.VersionedSample,
	sourceIndexes []int,
) error {
	match := 0
	for index := range count {
		value, err := reader.string("string value")
		if err != nil {
			return err
		}
		if match < len(sourceIndexes) && sourceIndexes[match] == index {
			samples[match].Value = model.StringValue(value)
			match++
		}
	}
	return nil
}

func matchingTimestampCount(timestamps []int64, query Query) int {
	count := 0
	for _, timestamp := range timestamps {
		if timestamp >= query.Start && timestamp <= query.End {
			count++
		}
	}
	return count
}

func fillAlignedSampleValues(
	reader *blockReader,
	fieldType model.FieldType,
	timestamps []int64,
	query Query,
	samples []model.VersionedSample,
) error {
	switch fieldType {
	case model.FieldFloat64:
		return fillAlignedFloatValues(reader, timestamps, query, samples)
	case model.FieldInt64:
		return fillAlignedIntValues(reader, timestamps, query, samples)
	case model.FieldBool:
		return fillAlignedBoolValues(reader, timestamps, query, samples)
	case model.FieldString:
		return fillAlignedStringValues(reader, timestamps, query, samples)
	default:
		return fmt.Errorf("unsupported value block field type %d", fieldType)
	}
}

func fillAlignedFloatValues(
	reader *blockReader,
	timestamps []int64,
	query Query,
	samples []model.VersionedSample,
) error {
	outIndex := 0
	for _, timestamp := range timestamps {
		value, err := reader.float64()
		if err != nil {
			return err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples[outIndex].Value = model.Float64Value(value)
		outIndex++
	}
	return nil
}

func fillAlignedIntValues(
	reader *blockReader,
	timestamps []int64,
	query Query,
	samples []model.VersionedSample,
) error {
	outIndex := 0
	for _, timestamp := range timestamps {
		value, err := reader.varint("int value")
		if err != nil {
			return err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples[outIndex].Value = model.Int64Value(value)
		outIndex++
	}
	return nil
}

func fillAlignedBoolValues(
	reader *blockReader,
	timestamps []int64,
	query Query,
	samples []model.VersionedSample,
) error {
	byteCount := (len(timestamps) + 7) / 8
	if len(reader.rest) < byteCount {
		return fmt.Errorf("read bool values: read bool bits: truncated payload")
	}
	bits := reader.rest[:byteCount]
	reader.rest = reader.rest[byteCount:]
	outIndex := 0
	for index, timestamp := range timestamps {
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		value := bits[index/8]&(1<<uint(index%8)) != 0
		samples[outIndex].Value = model.BoolValue(value)
		outIndex++
	}
	return nil
}

func fillAlignedStringValues(
	reader *blockReader,
	timestamps []int64,
	query Query,
	samples []model.VersionedSample,
) error {
	outIndex := 0
	for _, timestamp := range timestamps {
		value, err := reader.string("string value")
		if err != nil {
			return err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples[outIndex].Value = model.StringValue(value)
		outIndex++
	}
	return nil
}

func marshalValuePageIndex(dst []byte, index valuePageIndex) ([]byte, error) {
	dst = append(dst, valueEncodingPageIndex)
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

type valuePageIndexHeader struct {
	fieldID   uint32
	fieldType model.FieldType
	count     int
	pageCount int
}

var valuePageRefReadHook func()

func unmarshalValuePageIndex(payload []byte) (valuePageIndex, error) {
	reader := newBlockReader(payload)
	header, err := readValuePageIndexHeader(reader)
	if err != nil {
		return valuePageIndex{}, err
	}
	pages := make([]valuePageRef, 0, header.pageCount)
	for range header.pageCount {
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
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Count:     header.count,
		Pages:     pages,
	}, nil
}

func readValuePageIndexHeader(reader *blockReader) (valuePageIndexHeader, error) {
	encoding, err := reader.byte("value page index encoding")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	if encoding != valueEncodingPageIndex {
		return valuePageIndexHeader{}, fmt.Errorf("unknown value page index encoding %d", encoding)
	}
	fieldID64, err := reader.uvarint("value page index field id")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	fieldID, err := uint32Value("value page index field id", fieldID64)
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	fieldType, err := reader.byte("value page index field type")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	count, err := reader.intCount("value page index sample count")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	pageCount, err := reader.intCount("value page index page count")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	return valuePageIndexHeader{
		fieldID:   fieldID,
		fieldType: model.FieldType(fieldType),
		count:     count,
		pageCount: pageCount,
	}, nil
}

func readValuePageRef(reader *blockReader) (valuePageRef, error) {
	if valuePageRefReadHook != nil {
		valuePageRefReadHook()
	}
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
