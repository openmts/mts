package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/model"
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
	if len(samples) == 0 {
		return compressionPlain, nil, nil
	}
	selected := compressionPolicy(policy, "xor")
	if selected == "plain" {
		return compressionPlain, appendFloatValues(make([]byte, 0, len(samples)*8), samples), nil
	}
	plain := appendFloatValues(make([]byte, 0, len(samples)*8), samples)
	bestCodec := compressionPlain
	best := plain

	// 1) 等差 / index*scale（覆盖 scale 的 f0..f4）。读路径 O(窗口) 且载荷极小，优先选用。
	if payload, ok := encodeFloatConstStepPayload(samples); ok {
		return compressionConstStep, payload, nil
	}

	// 2) 整数值 float：复用 int delta / RLE。
	if intSamples, ok := floatSamplesAsIntSamples(samples); ok {
		delta := appendDeltaIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), intSamples)
		if len(delta) < len(best) {
			bestCodec = compressionDelta
			best = delta
		}
		rle := appendDeltaRLEIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), intSamples)
		if len(rle) < len(best) {
			bestCodec = compressionRLE
			best = rle
		}
	}

	// 3) Gorilla 位打包。
	gorilla := appendGorillaFloatValues(make([]byte, 0, gorillaFloatCapacity(len(samples))), samples)
	if len(gorilla) < len(best) {
		bestCodec = compressionXOR
		best = gorilla
	}
	return bestCodec, best, nil
}

func gorillaFloatCapacity(count int) int {
	if count <= 1 {
		return count * 8
	}
	// 首值 8 字节 + 最坏约每点 1+1+5+6+64 bit ≈ 10 字节。
	return 8 + (count-1)*10
}

// appendXORFloatValues 兼容旧测试名，实际为 Gorilla 位打包。
func appendXORFloatValues(dst []byte, samples []model.VersionedSample) []byte {
	return appendGorillaFloatValues(dst, samples)
}

func encodeIntValues(samples []model.VersionedSample, policy string) (byte, []byte, error) {
	plain := appendIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
	selected := compressionPolicy(policy, "delta")
	if selected == "plain" {
		return compressionPlain, plain, nil
	}
	// 选择 delta / delta-RLE 中更短者，再与 plain 比较。
	delta := appendDeltaIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
	rle := appendDeltaRLEIntValues(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
	bestCodec := compressionDelta
	best := delta
	if len(rle) < len(best) {
		bestCodec = compressionRLE
		best = rle
	}
	if len(best) < len(plain) {
		return bestCodec, best, nil
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
	// const-step + omitted writeSeq + 可随机访问的值编码：只解码查询窗口。
	if times.codecID == compressionConstStep {
		if samples, ok, err := readConstStepWindowSamples(fieldType, count, times, writeSeqs, values, query); err != nil {
			return nil, err
		} else if ok {
			return samples, nil
		}
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

func readConstStepWindowSamples(
	fieldType model.FieldType,
	count int,
	times codecPayload,
	writeSeqs codecPayload,
	values codecPayload,
	query Query,
) ([]model.VersionedSample, bool, error) {
	timestamps, start, err := decodeConstStepTimestampsWindow(times.payload, count, query)
	if err != nil {
		return nil, false, err
	}
	if len(timestamps) == 0 {
		return []model.VersionedSample{}, true, nil
	}
	// 仅 omitted writeSeq 可跳过全量解码；其它 writeSeq 编码仍走通用路径。
	if writeSeqs.codecID != compressionOmitted {
		return nil, false, nil
	}
	// 值侧：const-step/RLE/delta 支持按全局下标切片；Gorilla 依赖前缀状态，回退。
	switch fieldType {
	case model.FieldFloat64:
		if values.codecID != compressionConstStep && values.codecID != compressionDelta && values.codecID != compressionRLE {
			return nil, false, nil
		}
	case model.FieldInt64:
		if values.codecID != compressionDelta && values.codecID != compressionRLE {
			return nil, false, nil
		}
	default:
		return nil, false, nil
	}
	windowSeqs := make([]uint64, len(timestamps)) // omitted => 0
	// 为窗口解码构造“窗口对齐”的 timestamps，但值解码需要全局 index 语义。
	// 各 *SampleValues 用 enumerate index 对齐 timestamps/writeSeqs；因此传入窗口切片即可，
	// 但 const-step/float-int 的 index 必须是全局下标。这里单独走 window-aware 解码。
	samples, err := readWindowedCodecValues(fieldType, values, count, start, timestamps, windowSeqs, query)
	if err != nil {
		return nil, false, err
	}
	return samples, true, nil
}

func readWindowedCodecValues(
	fieldType model.FieldType,
	values codecPayload,
	fullCount int,
	start int,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	reader := newBlockReader(values.payload)
	var (
		samples []model.VersionedSample
		err     error
	)
	switch fieldType {
	case model.FieldFloat64:
		samples, err = readWindowedFloatValues(reader, values.codecID, fullCount, start, timestamps, writeSeqs, query)
	case model.FieldInt64:
		samples, err = readWindowedIntValues(reader, values.codecID, fullCount, start, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unsupported windowed field type %d", fieldType)
	}
	if err != nil {
		return nil, err
	}
	return samples, reader.done("values")
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
		return readCompressedFloatSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case model.FieldInt64:
		if codecID == compressionRLE {
			return readDeltaRLEIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
		}
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
		return readCompressedFloatValues(reader, codecID, count)
	case model.FieldInt64:
		if codecID == compressionRLE {
			return readDeltaRLEIntValues(reader, codecID, count)
		}
		return readDeltaIntValues(reader, codecID, count)
	case model.FieldString:
		return readDictionaryStringValues(reader, codecID, count)
	default:
		return nil, fmt.Errorf("unsupported value compression %d for field type %d", codecID, fieldType)
	}
}

func readCompressedFloatValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	switch codecID {
	case compressionXOR:
		return readGorillaFloatValues(reader, codecID, count)
	case compressionConstStep:
		return readFloatConstStepValues(reader, codecID, count)
	case compressionDelta, compressionRLE:
		return readFloatIntValues(reader, codecID, count)
	default:
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
}

func readCompressedFloatSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	switch codecID {
	case compressionXOR:
		return readGorillaFloatSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case compressionConstStep:
		return readFloatConstStepSampleValues(reader, codecID, timestamps, writeSeqs, query)
	case compressionDelta, compressionRLE:
		return readFloatIntSampleValues(reader, codecID, timestamps, writeSeqs, query)
	default:
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
}

// 兼容旧测试名。
func readXORFloatValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	return readCompressedFloatValues(reader, codecID, count)
}

func readXORFloatSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	return readCompressedFloatSampleValues(reader, codecID, timestamps, writeSeqs, query)
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
	dict, mode, err := readStringDictionary(reader)
	if err != nil {
		return nil, err
	}
	return readStringDictionaryOrdinals(reader, dict, mode, count)
}
