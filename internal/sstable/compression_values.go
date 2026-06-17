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
	plain := appendFloatValues(make([]byte, 0, len(samples)*8), samples)
	if compressionPolicy(policy, "xor") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendXORFloatValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
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
	plain := appendIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
	if compressionPolicy(policy, "delta") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDeltaIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
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
	plain := appendStringValues(make([]byte, 0, estimateStringValuesSize(samples)), samples)
	if compressionPolicy(policy, "dictionary") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDictionaryStringValues(make([]byte, 0, estimateStringValuesSize(samples)), samples)
	if len(candidate) < len(plain) {
		return compressionDictionary, candidate, nil
	}
	return compressionPlain, plain, nil
}

func estimateStringValuesSize(samples []model.VersionedSample) int {
	size := 0
	for _, sample := range samples {
		size += binary.MaxVarintLen64 + len(sample.Value.String)
	}
	return size
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

func readCodecSamples(
	reader *blockReader,
	fieldType model.FieldType,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	codecID, payload, err := readCodecPayload(reader, "values")
	if err != nil {
		return nil, err
	}
	payloadReader := newBlockReader(payload)
	var samples []model.VersionedSample
	if codecID == compressionPlain {
		samples, err = readSamples(payloadReader, fieldType, timestamps, writeSeqs, query)
	} else {
		var values []model.FieldValue
		values, err = readCompressedValues(payloadReader, fieldType, codecID, len(timestamps))
		if err == nil {
			header := valueHeader{fieldType: fieldType, count: len(timestamps)}
			samples = buildValueBlock(header, timestamps, writeSeqs, values).filter(query).Samples
		}
	}
	if err != nil {
		return nil, err
	}
	return samples, payloadReader.done("values")
}

func readCompressedSamples(
	fieldType model.FieldType,
	count int,
	times codecPayload,
	writeSeqs codecPayload,
	values codecPayload,
	query Query,
) ([]model.VersionedSample, error) {
	if times.codecID == compressionPlain &&
		writeSeqs.codecID == compressionPlain &&
		values.codecID == compressionPlain {
		return readPlainCompressedSamples(fieldType, count, times.payload, writeSeqs.payload, values.payload, query)
	}
	timestamps, err := decodeCodecTimestamps(times.codecID, times.payload, count)
	if err != nil {
		return nil, err
	}
	seqs, err := decodeCodecWriteSeqs(writeSeqs.codecID, writeSeqs.payload, count)
	if err != nil {
		return nil, err
	}
	payloadReader := blockReader{rest: values.payload}
	samples, err := readCodecPayloadSamples(&payloadReader, fieldType, values.codecID, timestamps, seqs, query)
	if err != nil {
		return nil, err
	}
	return samples, payloadReader.done("values")
}

func readCodecPayloadSamples(
	reader *blockReader,
	fieldType model.FieldType,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID == compressionPlain {
		return readSamples(reader, fieldType, timestamps, writeSeqs, query)
	}
	switch fieldType {
	case model.FieldFloat64:
		return readXORFloatSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case model.FieldInt64:
		return readDeltaIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case model.FieldString:
		return readDictionaryStringSampleValues(reader, codecID, timestamps, writeSeqs, query)
	}
	values, err := readCompressedValues(reader, fieldType, codecID, len(timestamps))
	if err != nil {
		return nil, err
	}
	header := valueHeader{fieldType: fieldType, count: len(timestamps)}
	return buildValueBlock(header, timestamps, writeSeqs, values).filter(query).Samples, nil
}

func readPlainCompressedSamples(
	fieldType model.FieldType,
	count int,
	timePayload []byte,
	writeSeqPayload []byte,
	valuePayload []byte,
	query Query,
) ([]model.VersionedSample, error) {
	timeReader := blockReader{rest: timePayload}
	seqReader := blockReader{rest: writeSeqPayload}
	valueReader := blockReader{rest: valuePayload}
	samples, err := readPlainCompressedSamplesByType(
		fieldType,
		count,
		&timeReader,
		&seqReader,
		&valueReader,
		query,
	)
	if err != nil {
		return nil, err
	}
	if err := timeReader.done("timestamps"); err != nil {
		return nil, err
	}
	if err := seqReader.done("write seqs"); err != nil {
		return nil, err
	}
	if err := valueReader.done("values"); err != nil {
		return nil, err
	}
	return samples, nil
}

func readPlainCompressedSamplesByType(
	fieldType model.FieldType,
	count int,
	timeReader *blockReader,
	seqReader *blockReader,
	valueReader *blockReader,
	query Query,
) ([]model.VersionedSample, error) {
	switch fieldType {
	case model.FieldFloat64:
		return readPlainCompressedFloatSamples(count, timeReader, seqReader, valueReader, query)
	case model.FieldInt64:
		return readPlainCompressedIntSamples(count, timeReader, seqReader, valueReader, query)
	case model.FieldBool:
		return readPlainCompressedBoolSamples(count, timeReader, seqReader, valueReader, query)
	case model.FieldString:
		return readPlainCompressedStringSamples(count, timeReader, seqReader, valueReader, query)
	default:
		return nil, fmt.Errorf("unsupported value block field type %d", fieldType)
	}
}

func readPlainCompressedFloatSamples(
	count int,
	timeReader *blockReader,
	seqReader *blockReader,
	valueReader *blockReader,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(count, query))
	var timestamp int64
	for index := range count {
		var err error
		timestamp, err = nextPlainTimestamp(timeReader, index, timestamp)
		if err != nil {
			return nil, err
		}
		writeSeq, err := seqReader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		value, err := valueReader.float64()
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeq,
			Value:     model.Float64Value(value),
		})
	}
	return samples, nil
}

func readPlainCompressedIntSamples(
	count int,
	timeReader *blockReader,
	seqReader *blockReader,
	valueReader *blockReader,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(count, query))
	var timestamp int64
	for index := range count {
		var err error
		timestamp, err = nextPlainTimestamp(timeReader, index, timestamp)
		if err != nil {
			return nil, err
		}
		writeSeq, err := seqReader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		value, err := valueReader.varint("int value")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeq,
			Value:     model.Int64Value(value),
		})
	}
	return samples, nil
}

func readPlainCompressedBoolSamples(
	count int,
	timeReader *blockReader,
	seqReader *blockReader,
	valueReader *blockReader,
	query Query,
) ([]model.VersionedSample, error) {
	byteCount := (count + 7) / 8
	if len(valueReader.rest) < byteCount {
		return nil, fmt.Errorf("read bool values: read bool bits: truncated payload")
	}
	bits := valueReader.rest[:byteCount]
	valueReader.rest = valueReader.rest[byteCount:]
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(count, query))
	var timestamp int64
	for index := range count {
		var err error
		timestamp, err = nextPlainTimestamp(timeReader, index, timestamp)
		if err != nil {
			return nil, err
		}
		writeSeq, err := seqReader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		value := bits[index/8]&(1<<uint(index%8)) != 0
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeq,
			Value:     model.BoolValue(value),
		})
	}
	return samples, nil
}

func readPlainCompressedStringSamples(
	count int,
	timeReader *blockReader,
	seqReader *blockReader,
	valueReader *blockReader,
	query Query,
) ([]model.VersionedSample, error) {
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(count, query))
	var timestamp int64
	for index := range count {
		var err error
		timestamp, err = nextPlainTimestamp(timeReader, index, timestamp)
		if err != nil {
			return nil, err
		}
		writeSeq, err := seqReader.uvarint("write seq")
		if err != nil {
			return nil, err
		}
		value, err := valueReader.string("string value")
		if err != nil {
			return nil, err
		}
		if timestamp < query.Start || timestamp > query.End {
			continue
		}
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  writeSeq,
			Value:     model.StringValue(value),
		})
	}
	return samples, nil
}

func nextPlainTimestamp(reader *blockReader, index int, previous int64) (int64, error) {
	if index == 0 {
		return reader.fixedInt64("first timestamp")
	}
	delta, err := reader.varint("timestamp delta")
	if err != nil {
		return 0, err
	}
	return previous + delta, nil
}

func compressedQueryCapacity(count int, query Query) int {
	if query.End < query.Start || count == 0 {
		return 0
	}
	width := query.End - query.Start + 1
	if width <= 0 || width > int64(count) {
		return count
	}
	return int(width)
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

func readXORFloatSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionXOR {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	if len(timestamps) == 0 {
		return samples, nil
	}
	first, err := reader.fixedInt64("first float bits")
	if err != nil {
		return nil, err
	}
	prev := uint64(first)
	samples = appendCompressedFloatSample(samples, timestamps[0], writeSeqs[0], prev, query)
	for index := 1; index < len(timestamps); index++ {
		xor, err := reader.uvarint("float xor")
		if err != nil {
			return nil, err
		}
		prev ^= xor
		samples = appendCompressedFloatSample(samples, timestamps[index], writeSeqs[index], prev, query)
	}
	return samples, nil
}

func appendCompressedFloatSample(
	samples []model.VersionedSample,
	timestamp int64,
	writeSeq uint64,
	bits uint64,
	query Query,
) []model.VersionedSample {
	if timestamp < query.Start || timestamp > query.End {
		return samples
	}
	return append(samples, model.VersionedSample{
		Timestamp: timestamp,
		WriteSeq:  writeSeq,
		Value:     model.Float64Value(math.Float64frombits(bits)),
	})
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

func readDeltaIntSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionDelta {
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
	for index := 1; index < len(timestamps); index++ {
		delta, err := reader.uvarint("int delta")
		if err != nil {
			return nil, err
		}
		prev += unzigZag64(delta)
		samples = appendCompressedIntSample(samples, timestamps[index], writeSeqs[index], prev, query)
	}
	return samples, nil
}

func appendCompressedIntSample(
	samples []model.VersionedSample,
	timestamp int64,
	writeSeq uint64,
	value int64,
	query Query,
) []model.VersionedSample {
	if timestamp < query.Start || timestamp > query.End {
		return samples
	}
	return append(samples, model.VersionedSample{
		Timestamp: timestamp,
		WriteSeq:  writeSeq,
		Value:     model.Int64Value(value),
	})
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
	dict, err := readStringDictionary(reader)
	if err != nil {
		return nil, err
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	for index, timestamp := range timestamps {
		ordinal, err := reader.intCount("string dictionary ordinal")
		if err != nil {
			return nil, err
		}
		if ordinal >= len(dict) {
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
