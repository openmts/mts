package wal

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"codeberg.org/mts/mts/internal/codec"
	"codeberg.org/mts/mts/internal/model"
)

const batchVersion byte = 2

func encodeBatch(records []model.ResolvedPoint) ([]byte, error) {
	dst := make([]byte, 0, estimateBatchSize(records))
	dst = append(dst, batchVersion)
	dst = binary.AppendUvarint(dst, uint64(len(records)))
	for _, record := range records {
		var err error
		dst, err = appendPoint(dst, record)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func estimateBatchSize(records []model.ResolvedPoint) int {
	size := 1 + uvarintSize(uint64(len(records)))
	for _, record := range records {
		size += estimatePointSize(record)
	}
	return size
}

func estimatePointSize(point model.ResolvedPoint) int {
	size := stringSize(point.Database)
	size += stringSize(point.RetentionPolicy)
	size += stringSize(point.Measurement)
	size += estimateTagsSize(point.Tags)
	size += uvarintSize(point.SeriesID)
	size += varintSize(point.Timestamp)
	size += uvarintSize(point.WriteSeq)
	size += uvarintSize(uint64(len(point.Fields)))
	for _, field := range point.Fields {
		size += estimateFieldSize(field)
	}
	return size
}

func estimateTagsSize(tags map[string]string) int {
	size := uvarintSize(uint64(len(tags)))
	for key, value := range tags {
		size += stringSize(key) + stringSize(value)
	}
	return size
}

func estimateFieldSize(field model.ResolvedField) int {
	size := uvarintSize(uint64(field.FieldID))
	size += stringSize(field.FieldName) + 1
	switch field.Value.Type {
	case model.FieldFloat64:
		return size + 8
	case model.FieldInt64:
		return size + varintSize(field.Value.Int64)
	case model.FieldString:
		return size + stringSize(field.Value.String)
	case model.FieldBool:
		return size + 1
	default:
		return size
	}
}

func stringSize(value string) int {
	return uvarintSize(uint64(len(value))) + len(value)
}

func uvarintSize(value uint64) int {
	size := 1
	for value >= 0x80 {
		value >>= 7
		size++
	}
	return size
}

func varintSize(value int64) int {
	encoded := uint64(value) << 1
	if value < 0 {
		encoded = ^encoded
	}
	return uvarintSize(encoded)
}

func decodeBatch(payload []byte) ([]model.ResolvedPoint, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("missing batch version")
	}
	if payload[0] != batchVersion {
		return nil, fmt.Errorf("unsupported batch version %d", payload[0])
	}
	reader := newBatchReader(payload[1:])
	count, err := reader.intCount("point count")
	if err != nil {
		return nil, err
	}
	records := make([]model.ResolvedPoint, 0, count)
	for range count {
		record, err := readPoint(reader)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := reader.done("wal batch"); err != nil {
		return nil, err
	}
	return records, nil
}

func appendPoint(dst []byte, point model.ResolvedPoint) ([]byte, error) {
	dst = codec.AppendString(dst, point.Database)
	dst = codec.AppendString(dst, point.RetentionPolicy)
	dst = codec.AppendString(dst, point.Measurement)
	dst = appendTags(dst, point.Tags)
	dst = binary.AppendUvarint(dst, point.SeriesID)
	dst = binary.AppendVarint(dst, point.Timestamp)
	dst = binary.AppendUvarint(dst, point.WriteSeq)
	dst = binary.AppendUvarint(dst, uint64(len(point.Fields)))
	for _, field := range point.Fields {
		var err error
		dst, err = appendField(dst, field)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func readPoint(reader *batchReader) (model.ResolvedPoint, error) {
	identity, err := readPointIdentity(reader)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	seriesID, timestamp, writeSeq, err := readPointVersion(reader)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	fields, err := readFields(reader)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	identity.SeriesID = seriesID
	identity.Timestamp = timestamp
	identity.WriteSeq = writeSeq
	identity.Fields = fields
	return identity, nil
}

func readPointIdentity(reader *batchReader) (model.ResolvedPoint, error) {
	database, err := reader.string("database")
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	policy, err := reader.string("retention policy")
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	measurement, err := reader.string("measurement")
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	tags, err := readTags(reader)
	return model.ResolvedPoint{Database: database, RetentionPolicy: policy, Measurement: measurement, Tags: tags}, err
}

func readPointVersion(reader *batchReader) (uint64, int64, uint64, error) {
	seriesID, err := reader.uvarint("series id")
	if err != nil {
		return 0, 0, 0, err
	}
	timestamp, err := reader.varint("timestamp")
	if err != nil {
		return 0, 0, 0, err
	}
	writeSeq, err := reader.uvarint("write seq")
	return seriesID, timestamp, writeSeq, err
}

func appendField(dst []byte, field model.ResolvedField) ([]byte, error) {
	if field.Type != field.Value.Type {
		return nil, fmt.Errorf("field %s type %d does not match value type %d", field.FieldName, field.Type, field.Value.Type)
	}
	dst = binary.AppendUvarint(dst, uint64(field.FieldID))
	dst = codec.AppendString(dst, field.FieldName)
	dst = append(dst, byte(field.Type))
	return appendValuePayload(dst, field.Value)
}

func readFields(reader *batchReader) ([]model.ResolvedField, error) {
	count, err := reader.intCount("field count")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	fields := make([]model.ResolvedField, 0, count)
	for range count {
		field, err := readField(reader)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func readField(reader *batchReader) (model.ResolvedField, error) {
	id, err := reader.uvarint("field id")
	if err != nil {
		return model.ResolvedField{}, err
	}
	fieldID, err := uint32Value("field id", id)
	if err != nil {
		return model.ResolvedField{}, err
	}
	name, err := reader.string("field name")
	if err != nil {
		return model.ResolvedField{}, err
	}
	fieldType, err := reader.byte("field type")
	if err != nil {
		return model.ResolvedField{}, err
	}
	value, err := readValuePayload(model.FieldType(fieldType), reader)
	if err != nil {
		return model.ResolvedField{}, err
	}
	return model.ResolvedField{FieldID: fieldID, FieldName: name, Type: model.FieldType(fieldType), Value: value}, nil
}

func appendTags(dst []byte, tags map[string]string) []byte {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	dst = binary.AppendUvarint(dst, uint64(len(keys)))
	for _, key := range keys {
		dst = codec.AppendString(dst, key)
		dst = codec.AppendString(dst, tags[key])
	}
	return dst
}

func readTags(reader *batchReader) (map[string]string, error) {
	count, err := reader.intCount("tag count")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	tags := make(map[string]string, count)
	for range count {
		key, value, err := readTagPair(reader)
		if err != nil {
			return nil, err
		}
		tags[key] = value
	}
	return tags, nil
}

func readTagPair(reader *batchReader) (string, string, error) {
	key, err := reader.string("tag key")
	if err != nil {
		return "", "", err
	}
	value, err := reader.string("tag value")
	if err != nil {
		return "", "", err
	}
	return key, value, nil
}

func appendValuePayload(dst []byte, value model.FieldValue) ([]byte, error) {
	switch value.Type {
	case model.FieldFloat64:
		return binary.LittleEndian.AppendUint64(dst, math.Float64bits(value.Float64)), nil
	case model.FieldInt64:
		return binary.AppendVarint(dst, value.Int64), nil
	case model.FieldString:
		return codec.AppendString(dst, value.String), nil
	case model.FieldBool:
		if value.Bool {
			return append(dst, 1), nil
		}
		return append(dst, 0), nil
	default:
		return nil, fmt.Errorf("unsupported field value type %d", value.Type)
	}
}

func readValuePayload(fieldType model.FieldType, reader *batchReader) (model.FieldValue, error) {
	switch fieldType {
	case model.FieldFloat64:
		return reader.float64()
	case model.FieldInt64:
		value, err := reader.varint("int64 field value")
		return model.Int64Value(value), err
	case model.FieldString:
		value, err := reader.string("string field value")
		return model.StringValue(value), err
	case model.FieldBool:
		value, err := reader.bool("bool field value")
		return model.BoolValue(value), err
	default:
		return model.FieldValue{}, fmt.Errorf("unsupported field value type %d", fieldType)
	}
}

type batchReader struct {
	rest []byte
}

func newBatchReader(data []byte) *batchReader {
	return &batchReader{rest: data}
}

func (r *batchReader) uvarint(name string) (uint64, error) {
	value, size := binary.Uvarint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode wal %s: invalid uvarint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}

func (r *batchReader) varint(name string) (int64, error) {
	value, size := binary.Varint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode wal %s: invalid varint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}

func (r *batchReader) intCount(name string) (int, error) {
	value, err := r.uvarint(name)
	if err != nil {
		return 0, err
	}
	maxInt := uint64(int(^uint(0) >> 1))
	if value > maxInt {
		return 0, fmt.Errorf("decode wal %s: count %d overflows int", name, value)
	}
	return int(value), nil
}

func (r *batchReader) string(name string) (string, error) {
	value, rest, err := codec.ReadString(r.rest)
	if err != nil {
		return "", fmt.Errorf("decode wal %s: %w", name, err)
	}
	r.rest = rest
	return value, nil
}

func (r *batchReader) byte(name string) (byte, error) {
	if len(r.rest) == 0 {
		return 0, fmt.Errorf("decode wal %s: missing byte", name)
	}
	value := r.rest[0]
	r.rest = r.rest[1:]
	return value, nil
}

func (r *batchReader) bool(name string) (bool, error) {
	value, err := r.byte(name)
	if err != nil {
		return false, err
	}
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("decode wal %s: invalid bool byte %d", name, value)
	}
}

func (r *batchReader) float64() (model.FieldValue, error) {
	if len(r.rest) < 8 {
		return model.FieldValue{}, fmt.Errorf("decode wal float64 field value: truncated payload")
	}
	value := math.Float64frombits(binary.LittleEndian.Uint64(r.rest[:8]))
	r.rest = r.rest[8:]
	return model.Float64Value(value), nil
}

func (r *batchReader) done(name string) error {
	if len(r.rest) != 0 {
		return fmt.Errorf("decode %s: %d trailing bytes", name, len(r.rest))
	}
	return nil
}

func uint32Value(name string, value uint64) (uint32, error) {
	if value > uint64(^uint32(0)) {
		return 0, fmt.Errorf("decode wal %s: value %d overflows uint32", name, value)
	}
	return uint32(value), nil
}
