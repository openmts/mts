package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

const (
	timeEncodingDelta           byte = 1
	timeEncodingConstStep       byte = 2
	valueEncodingPagePlain      byte = 3
	valueEncodingPageIndex      byte = 4
	valueEncodingPageCompressed byte = 5

	timeRefModeAligned byte = 0
	timeRefModeIndexed byte = 1

	valuePageStatsNumeric byte = 1
)

func marshalTimeBlock(dst []byte, timestamps []int64) []byte {
	if step, ok := detectRowTimeConstStep(timestamps); ok {
		dst = append(dst, timeEncodingConstStep)
		dst = binary.AppendUvarint(dst, uint64(len(timestamps)))
		if len(timestamps) == 0 {
			return dst
		}
		dst = binary.LittleEndian.AppendUint64(dst, uint64(timestamps[0]))
		return binary.AppendVarint(dst, step)
	}
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

func detectRowTimeConstStep(timestamps []int64) (int64, bool) {
	if len(timestamps) < 2 {
		return 0, false
	}
	step := timestamps[1] - timestamps[0]
	for index := 2; index < len(timestamps); index++ {
		if timestamps[index]-timestamps[index-1] != step {
			return 0, false
		}
	}
	return step, true
}

func unmarshalTimeBlock(payload []byte) ([]int64, error) {
	reader := newBlockReader(payload)
	encoding, err := reader.byte("time encoding")
	if err != nil {
		return nil, err
	}
	count, err := reader.intCount("time count")
	if err != nil {
		return nil, err
	}
	switch encoding {
	case timeEncodingDelta:
		timestamps, err := readTimestamps(reader, count)
		if err != nil {
			return nil, err
		}
		return timestamps, reader.done("time block")
	case timeEncodingConstStep:
		timestamps, err := readConstStepTimestamps(reader, count)
		if err != nil {
			return nil, err
		}
		return timestamps, reader.done("time block")
	default:
		return nil, fmt.Errorf("unknown time encoding %d", encoding)
	}
}

func readConstStepTimestamps(reader *blockReader, count int) ([]int64, error) {
	if count == 0 {
		return nil, nil
	}
	base, err := reader.fixedInt64("const-step time base")
	if err != nil {
		return nil, err
	}
	step, err := reader.varint("const-step time step")
	if err != nil {
		return nil, err
	}
	timestamps := make([]int64, count)
	for index := range timestamps {
		timestamps[index] = base + int64(index)*step
	}
	return timestamps, nil
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
	if len(timestamps) == 0 || query.End < query.Start {
		return 0
	}
	// 行时间戳块按时间有序，二分窗口。
	lo, hi := sortedTimestampWindow(timestamps, query)
	return hi - lo
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
