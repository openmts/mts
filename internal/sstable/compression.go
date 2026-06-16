package sstable

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/mts/mts/internal/model"
)

const (
	compressionPlain byte = iota
	compressionDeltaOfDelta
	compressionXOR
	compressionDelta
	compressionDictionary
)

const defaultCompressionMinPageValues = 8

func marshalValuePage(
	dst []byte,
	column model.ColumnData,
	rowTimestamps []int64,
	opts model.CompressionOptions,
) ([]byte, error) {
	if !compressionEnabled(opts, len(column.Samples)) {
		return marshalValueBlockWithTimestamps(dst, column, rowTimestamps)
	}
	return marshalCompressedValueBlock(dst, column, opts)
}

func compressionEnabled(opts model.CompressionOptions, count int) bool {
	if !opts.Enabled {
		return false
	}
	minValues := opts.MinPageValues
	if minValues <= 0 {
		minValues = defaultCompressionMinPageValues
	}
	return count >= minValues
}

func marshalCompressedValueBlock(
	dst []byte,
	column model.ColumnData,
	opts model.CompressionOptions,
) ([]byte, error) {
	timestamps, writeSeqs := splitSampleMetadata(column.Samples)
	timeCodec, timePayload, err := encodeTimestamps(timestamps, opts.Timestamp)
	if err != nil {
		return nil, err
	}
	valueCodec, valuePayload, err := encodeTypedValues(column, opts)
	if err != nil {
		return nil, err
	}
	dst = append(dst, valueEncodingV5)
	dst = binary.AppendUvarint(dst, uint64(column.FieldID))
	dst = append(dst, byte(column.FieldType))
	dst = binary.AppendUvarint(dst, uint64(len(column.Samples)))
	dst = appendCodecPayload(dst, timeCodec, timePayload)
	dst = appendWriteSeqsPayload(dst, writeSeqs)
	dst = appendCodecPayload(dst, valueCodec, valuePayload)
	return dst, nil
}

func unmarshalCompressedValueBlock(payload []byte, query Query) (valueBlock, error) {
	reader := newBlockReader(payload)
	header, err := readValueHeaderV5(reader)
	if err != nil {
		return valueBlock{}, err
	}
	timestamps, err := readCodecTimestamps(reader, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	writeSeqs, err := readCodecWriteSeqs(reader, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	values, err := readCodecValues(reader, header.fieldType, header.count)
	if err != nil {
		return valueBlock{}, err
	}
	if err := reader.done("value block v5"); err != nil {
		return valueBlock{}, err
	}
	return buildValueBlock(header, timestamps, writeSeqs, values).filter(query), nil
}

func readValueHeaderV5(reader *blockReader) (valueHeader, error) {
	encoding, err := reader.byte("value encoding")
	if err != nil {
		return valueHeader{}, err
	}
	if encoding != valueEncodingV5 {
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

func (b valueBlock) filter(query Query) valueBlock {
	b.Samples = filterSamplesByTime(b.Samples, query)
	return b
}

func filterSamplesByTime(samples []model.VersionedSample, query Query) []model.VersionedSample {
	out := samples[:0]
	for _, sample := range samples {
		if sample.Timestamp >= query.Start && sample.Timestamp <= query.End {
			out = append(out, sample)
		}
	}
	return out
}

func splitSampleMetadata(samples []model.VersionedSample) ([]int64, []uint64) {
	timestamps := make([]int64, len(samples))
	writeSeqs := make([]uint64, len(samples))
	for index, sample := range samples {
		timestamps[index] = sample.Timestamp
		writeSeqs[index] = sample.WriteSeq
	}
	return timestamps, writeSeqs
}

func appendCodecPayload(dst []byte, codec byte, payload []byte) []byte {
	dst = append(dst, codec)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	return append(dst, payload...)
}

func appendWriteSeqsPayload(dst []byte, writeSeqs []uint64) []byte {
	payload := make([]byte, 0, len(writeSeqs))
	for _, seq := range writeSeqs {
		payload = binary.AppendUvarint(payload, seq)
	}
	return appendCodecPayload(dst, compressionPlain, payload)
}

func readCodecPayload(reader *blockReader, name string) (byte, []byte, error) {
	codecID, err := reader.byte(name + " codec")
	if err != nil {
		return 0, nil, err
	}
	size, err := reader.intCount(name + " size")
	if err != nil {
		return 0, nil, err
	}
	if size > len(reader.rest) {
		return 0, nil, fmt.Errorf("decode sstable %s: truncated payload", name)
	}
	payload := reader.rest[:size]
	reader.rest = reader.rest[size:]
	return codecID, payload, nil
}

func readCodecWriteSeqs(reader *blockReader, count int) ([]uint64, error) {
	codecID, payload, err := readCodecPayload(reader, "write seqs")
	if err != nil {
		return nil, err
	}
	if codecID != compressionPlain {
		return nil, fmt.Errorf("unknown write seq compression %d", codecID)
	}
	payloadReader := newBlockReader(payload)
	writeSeqs, err := readWriteSeqs(payloadReader, count)
	if err != nil {
		return nil, err
	}
	return writeSeqs, payloadReader.done("write seqs")
}

func compressionPolicy(policy string, defaultPolicy string) string {
	switch policy {
	case "", "auto":
		return defaultPolicy
	case "plain", defaultPolicy:
		return policy
	default:
		return defaultPolicy
	}
}

func zigZag64(value int64) uint64 {
	return uint64(value<<1) ^ uint64(value>>63)
}

func unzigZag64(value uint64) int64 {
	return int64(value>>1) ^ -int64(value&1)
}
