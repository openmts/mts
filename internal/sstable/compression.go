package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
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
	budget CompressionMemoryBudget,
) ([]byte, error) {
	if !compressionEnabled(opts, len(column.Samples)) {
		return marshalValueBlockWithTimestamps(dst, column, rowTimestamps)
	}
	return marshalCompressedValueBlock(dst, column, opts, budget)
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
	budget ...CompressionMemoryBudget,
) ([]byte, error) {
	timeCodec, timePayload, err := encodeSampleTimestamps(column.Samples, opts.Timestamp)
	if err != nil {
		return nil, err
	}
	valueCodec, valuePayload, err := encodeTypedValues(column, opts)
	if err != nil {
		return nil, err
	}
	dst = append(dst, valueEncodingPageCompressed)
	dst = binary.AppendUvarint(dst, uint64(column.FieldID))
	dst = append(dst, byte(column.FieldType))
	dst = binary.AppendUvarint(dst, uint64(len(column.Samples)))
	dst, err = appendCodecPayloadWithCompression(dst, timeCodec, timePayload, opts.Algorithm, budget...)
	if err != nil {
		return nil, err
	}
	dst, err = appendSampleWriteSeqsPayloadWithCompression(dst, column.Samples, opts.Algorithm, budget...)
	if err != nil {
		return nil, err
	}
	return appendCodecPayloadWithCompression(dst, valueCodec, valuePayload, opts.Algorithm, budget...)
}

func unmarshalCompressedValueBlock(payload []byte, query Query) (valueBlock, error) {
	reader := newBlockReader(payload)
	header, err := readCompressedValueHeader(reader)
	if err != nil {
		return valueBlock{}, err
	}
	timeCodec, timePayload, err := readCodecPayload(reader, "timestamps")
	if err != nil {
		return valueBlock{}, err
	}
	writeSeqCodec, writeSeqPayload, err := readCodecPayload(reader, "write seqs")
	if err != nil {
		return valueBlock{}, err
	}
	valueCodec, valuePayload, err := readCodecPayload(reader, "values")
	if err != nil {
		return valueBlock{}, err
	}
	if err := reader.done("compressed value page"); err != nil {
		return valueBlock{}, err
	}
	samples, err := readCompressedSamples(
		header.fieldType,
		header.count,
		codecPayload{codecID: timeCodec, payload: timePayload},
		codecPayload{codecID: writeSeqCodec, payload: writeSeqPayload},
		codecPayload{codecID: valueCodec, payload: valuePayload},
		query,
	)
	if err != nil {
		return valueBlock{}, err
	}
	return valueBlock{
		Encoding:  "compressed-values",
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Samples:   samples,
	}, nil
}

func readCompressedValueHeader(reader *blockReader) (valueHeader, error) {
	encoding, err := reader.byte("value encoding")
	if err != nil {
		return valueHeader{}, err
	}
	if encoding != valueEncodingPageCompressed {
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
	b.Samples = filterSamplesByFieldPredicates(b.FieldID, b.Samples, query)
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

func filterSamplesByFieldPredicates(
	fieldID uint32,
	samples []model.VersionedSample,
	query Query,
) []model.VersionedSample {
	predicates := query.FieldPredicates[fieldID]
	if len(predicates) == 0 {
		return samples
	}
	out := samples[:0]
	for _, sample := range samples {
		if sampleMatchesFieldPredicates(sample, predicates) {
			out = append(out, sample)
		}
	}
	return out
}

func sampleMatchesFieldPredicates(
	sample model.VersionedSample,
	predicates []model.QueryPredicate,
) bool {
	for _, predicate := range predicates {
		if !sampleMatchesFieldPredicate(sample.Value, predicate) {
			return false
		}
	}
	return true
}

func sampleMatchesFieldPredicate(value model.FieldValue, predicate model.QueryPredicate) bool {
	comparison := compareSampleFieldValue(value, predicate.Value)
	switch predicate.Kind {
	case model.QueryPredicateFieldEq:
		return comparison == 0
	case model.QueryPredicateFieldNe:
		return comparison != 0
	case model.QueryPredicateFieldGT:
		return comparison > 0
	case model.QueryPredicateFieldGTE:
		return comparison >= 0
	case model.QueryPredicateFieldLT:
		return comparison < 0
	case model.QueryPredicateFieldLTE:
		return comparison <= 0
	default:
		return true
	}
}

func compareSampleFieldValue(left model.FieldValue, right model.FieldValue) int {
	if numericSampleValue(left) && numericSampleValue(right) {
		return compareSampleFloat(sampleValueAsFloat(left), sampleValueAsFloat(right))
	}
	if left.Type != right.Type {
		return -1
	}
	switch left.Type {
	case model.FieldString:
		return compareSampleString(left.String, right.String)
	case model.FieldBool:
		return compareSampleBool(left.Bool, right.Bool)
	default:
		return -1
	}
}

func numericSampleValue(value model.FieldValue) bool {
	return value.Type == model.FieldFloat64 || value.Type == model.FieldInt64
}

func sampleValueAsFloat(value model.FieldValue) float64 {
	if value.Type == model.FieldFloat64 {
		return value.Float64
	}
	return float64(value.Int64)
}

func compareSampleFloat(left float64, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareSampleString(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareSampleBool(left bool, right bool) int {
	switch {
	case left == right:
		return 0
	case !left && right:
		return -1
	default:
		return 1
	}
}

func appendCodecPayload(dst []byte, codec byte, payload []byte) []byte {
	dst = append(dst, codec, payloadCompressionNone)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
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

func appendSampleWriteSeqsPayload(dst []byte, samples []model.VersionedSample) []byte {
	payload := make([]byte, 0, len(samples)*binary.MaxVarintLen64)
	for _, sample := range samples {
		payload = binary.AppendUvarint(payload, sample.WriteSeq)
	}
	return appendCodecPayload(dst, compressionPlain, payload)
}

func appendSampleWriteSeqsPayloadWithCompression(
	dst []byte,
	samples []model.VersionedSample,
	algorithm string,
	budget ...CompressionMemoryBudget,
) ([]byte, error) {
	payload := make([]byte, 0, len(samples)*binary.MaxVarintLen64)
	for _, sample := range samples {
		payload = binary.AppendUvarint(payload, sample.WriteSeq)
	}
	return appendCodecPayloadWithCompression(dst, compressionPlain, payload, algorithm, budget...)
}

func readCodecPayload(reader *blockReader, name string) (byte, []byte, error) {
	if len(reader.rest) == 0 {
		return 0, nil, fmt.Errorf("decode sstable %s codec: missing byte", name)
	}
	codecID := reader.rest[0]
	reader.rest = reader.rest[1:]
	if len(reader.rest) == 0 {
		return 0, nil, fmt.Errorf("decode sstable %s payload compression: missing byte", name)
	}
	algorithmID := reader.rest[0]
	reader.rest = reader.rest[1:]
	rawSize, err := readPayloadSize(reader, name, "raw")
	if err != nil {
		return 0, nil, err
	}
	storedSize, err := readPayloadSize(reader, name, "stored")
	if err != nil {
		return 0, nil, err
	}
	if storedSize > len(reader.rest) {
		return 0, nil, fmt.Errorf("decode sstable %s: truncated payload", name)
	}
	payload := reader.rest[:storedSize]
	reader.rest = reader.rest[storedSize:]
	decoded, err := decompressPayload(algorithmID, payload, rawSize)
	if err != nil {
		return 0, nil, fmt.Errorf("decode sstable %s payload: %w", name, err)
	}
	return codecID, decoded, nil
}

type codecPayload struct {
	codecID byte
	payload []byte
}

func readCodecWriteSeqs(reader *blockReader, count int) ([]uint64, error) {
	codecID, payload, err := readCodecPayload(reader, "write seqs")
	if err != nil {
		return nil, err
	}
	return decodeCodecWriteSeqs(codecID, payload, count)
}

func decodeCodecWriteSeqs(codecID byte, payload []byte, count int) ([]uint64, error) {
	if codecID != compressionPlain {
		return nil, fmt.Errorf("unknown write seq compression %d", codecID)
	}
	payloadReader := blockReader{rest: payload}
	writeSeqs, err := readWriteSeqs(&payloadReader, count)
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
