package wal

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

func encodeBatch(records []model.ResolvedPoint) ([]byte, error) {
	identities, identityRefs := batchIdentities(records)
	fieldNames, fieldNameRefs := batchFieldNames(records)
	dst := make([]byte, 0, estimateBatchSize(records))
	dst = binary.AppendUvarint(dst, uint64(len(identities)))
	for _, identity := range identities {
		dst = appendIdentity(dst, identity)
	}
	dst = binary.AppendUvarint(dst, uint64(len(fieldNames)))
	for _, name := range fieldNames {
		dst = codec.AppendString(dst, name)
	}
	dst = binary.AppendUvarint(dst, uint64(len(records)))
	for index, record := range records {
		var err error
		dst, err = appendPoint(dst, record, identityRefs[index], fieldNameRefs[index])
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

type batchIdentity struct {
	database        string
	retentionPolicy string
	measurement     string
	tags            map[string]string
}

func batchIdentities(records []model.ResolvedPoint) ([]batchIdentity, []int) {
	identities := make([]batchIdentity, 0)
	refs := make([]int, len(records))
	seen := make(map[string]int)
	keyScratch := make([]byte, 0)
	for index, record := range records {
		if ref, ok := lastIdentityRef(record, identities); ok {
			refs[index] = ref
			continue
		}
		var key string
		key, keyScratch = identityKeyWithScratch(record, keyScratch[:0])
		ref, ok := seen[key]
		if !ok {
			ref = len(identities)
			seen[key] = ref
			identities = append(identities, batchIdentity{
				database:        record.Database,
				retentionPolicy: record.RetentionPolicy,
				measurement:     record.Measurement,
				tags:            record.Tags,
			})
		}
		refs[index] = ref
	}
	return identities, refs
}

func lastIdentityRef(point model.ResolvedPoint, identities []batchIdentity) (int, bool) {
	if len(identities) == 0 {
		return 0, false
	}
	ref := len(identities) - 1
	return ref, identityMatches(point, identities[ref])
}

func identityMatches(point model.ResolvedPoint, identity batchIdentity) bool {
	if point.Database != identity.database ||
		point.RetentionPolicy != identity.retentionPolicy ||
		point.Measurement != identity.measurement ||
		len(point.Tags) != len(identity.tags) {
		return false
	}
	for key, value := range point.Tags {
		if identity.tags[key] != value {
			return false
		}
	}
	return true
}

func identityKeyWithScratch(point model.ResolvedPoint, dst []byte) (string, []byte) {
	if cap(dst) == 0 {
		dst = make([]byte, 0, stringSize(point.Database)+stringSize(point.RetentionPolicy)+stringSize(point.Measurement)+estimateTagsSize(point.Tags))
	}
	dst = codec.AppendString(dst, point.Database)
	dst = codec.AppendString(dst, point.RetentionPolicy)
	dst = codec.AppendString(dst, point.Measurement)
	dst = appendTags(dst, point.Tags)
	return string(dst), dst
}

func batchFieldNames(records []model.ResolvedPoint) ([]string, [][]int) {
	names := make([]string, 0)
	refs := make([][]int, len(records))
	arena := make([]int, totalFieldCount(records))
	offset := 0
	seen := make(map[string]int)
	for pointIndex, record := range records {
		pointRefs := arena[offset : offset+len(record.Fields)]
		offset += len(record.Fields)
		for fieldIndex, field := range record.Fields {
			ref, ok := seen[field.FieldName]
			if !ok {
				ref = len(names)
				seen[field.FieldName] = ref
				names = append(names, field.FieldName)
			}
			pointRefs[fieldIndex] = ref
		}
		refs[pointIndex] = pointRefs
	}
	return names, refs
}

func totalFieldCount(records []model.ResolvedPoint) int {
	total := 0
	for _, record := range records {
		total += len(record.Fields)
	}
	return total
}

func appendIdentity(dst []byte, identity batchIdentity) []byte {
	dst = codec.AppendString(dst, identity.database)
	dst = codec.AppendString(dst, identity.retentionPolicy)
	dst = codec.AppendString(dst, identity.measurement)
	return appendTags(dst, identity.tags)
}

func estimateBatchSize(records []model.ResolvedPoint) int {
	size := uvarintSize(uint64(len(records)))
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
		return nil, fmt.Errorf("empty wal batch")
	}
	reader := newBatchReader(payload)
	identities, err := readBatchIdentities(reader)
	if err != nil {
		return nil, err
	}
	fieldNames, err := readBatchFieldNames(reader)
	if err != nil {
		return nil, err
	}
	count, err := reader.intCount("point count")
	if err != nil {
		return nil, err
	}
	records := make([]model.ResolvedPoint, 0, count)
	for range count {
		record, err := readPoint(reader, identities, fieldNames)
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

func readBatchIdentities(reader *batchReader) ([]batchIdentity, error) {
	count, err := reader.intCount("identity count")
	if err != nil {
		return nil, err
	}
	identities := make([]batchIdentity, count)
	for index := range count {
		identity, err := readIdentity(reader)
		if err != nil {
			return nil, err
		}
		identities[index] = identity
	}
	return identities, nil
}

func readIdentity(reader *batchReader) (batchIdentity, error) {
	database, err := reader.string("database")
	if err != nil {
		return batchIdentity{}, err
	}
	policy, err := reader.string("retention policy")
	if err != nil {
		return batchIdentity{}, err
	}
	measurement, err := reader.string("measurement")
	if err != nil {
		return batchIdentity{}, err
	}
	tags, err := readTags(reader)
	return batchIdentity{
		database:        database,
		retentionPolicy: policy,
		measurement:     measurement,
		tags:            tags,
	}, err
}

func readBatchFieldNames(reader *batchReader) ([]string, error) {
	count, err := reader.intCount("field name count")
	if err != nil {
		return nil, err
	}
	names := make([]string, count)
	for index := range count {
		name, err := reader.string("field name")
		if err != nil {
			return nil, err
		}
		names[index] = name
	}
	return names, nil
}

func encodeTombstones(tombstones []model.Tombstone) ([]byte, error) {
	dst := make([]byte, 0, estimateTombstonesSize(tombstones))
	dst = binary.AppendUvarint(dst, uint64(len(tombstones)))
	for _, tombstone := range tombstones {
		dst = appendTombstone(dst, tombstone)
	}
	return dst, nil
}

func estimateTombstonesSize(tombstones []model.Tombstone) int {
	size := uvarintSize(uint64(len(tombstones)))
	for _, tombstone := range tombstones {
		size += varintSize(tombstone.StartTime) + varintSize(tombstone.EndTime)
		size += uvarintSize(tombstone.WriteSeq)
		size += uvarintSize(uint64(len(tombstone.SeriesIDs))) + len(tombstone.SeriesIDs)*10
		size += uvarintSize(uint64(len(tombstone.FieldIDs))) + len(tombstone.FieldIDs)*5
	}
	return size
}

func appendTombstone(dst []byte, tombstone model.Tombstone) []byte {
	dst = binary.AppendVarint(dst, tombstone.StartTime)
	dst = binary.AppendVarint(dst, tombstone.EndTime)
	dst = binary.AppendUvarint(dst, tombstone.WriteSeq)
	dst = binary.AppendUvarint(dst, uint64(len(tombstone.SeriesIDs)))
	for _, seriesID := range tombstone.SeriesIDs {
		dst = binary.AppendUvarint(dst, seriesID)
	}
	dst = binary.AppendUvarint(dst, uint64(len(tombstone.FieldIDs)))
	for _, fieldID := range tombstone.FieldIDs {
		dst = binary.AppendUvarint(dst, uint64(fieldID))
	}
	return dst
}

func decodeTombstones(payload []byte) ([]model.Tombstone, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty wal tombstones")
	}
	reader := newBatchReader(payload)
	count, err := reader.intCount("tombstone count")
	if err != nil {
		return nil, err
	}
	tombstones := make([]model.Tombstone, 0, count)
	for range count {
		tombstone, err := readTombstone(reader)
		if err != nil {
			return nil, err
		}
		tombstones = append(tombstones, tombstone)
	}
	if err := reader.done("wal tombstones"); err != nil {
		return nil, err
	}
	return tombstones, nil
}

func readTombstone(reader *batchReader) (model.Tombstone, error) {
	start, err := reader.varint("tombstone start")
	if err != nil {
		return model.Tombstone{}, err
	}
	end, err := reader.varint("tombstone end")
	if err != nil {
		return model.Tombstone{}, err
	}
	writeSeq, err := reader.uvarint("tombstone write seq")
	if err != nil {
		return model.Tombstone{}, err
	}
	seriesIDs, err := readUint64s(reader, "tombstone series")
	if err != nil {
		return model.Tombstone{}, err
	}
	fieldIDs, err := readUint32s(reader, "tombstone fields")
	return model.Tombstone{
		SeriesIDs: seriesIDs,
		FieldIDs:  fieldIDs,
		StartTime: start,
		EndTime:   end,
		WriteSeq:  writeSeq,
	}, err
}

func readUint64s(reader *batchReader, name string) ([]uint64, error) {
	count, err := reader.intCount(name + " count")
	if err != nil {
		return nil, err
	}
	values := make([]uint64, count)
	for index := range count {
		values[index], err = reader.uvarint(name + " value")
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func readUint32s(reader *batchReader, name string) ([]uint32, error) {
	count, err := reader.intCount(name + " count")
	if err != nil {
		return nil, err
	}
	values := make([]uint32, count)
	for index := range count {
		value, err := reader.uvarint(name + " value")
		if err != nil {
			return nil, err
		}
		values[index], err = uint32Value(name+" value", value)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func appendPoint(
	dst []byte,
	point model.ResolvedPoint,
	identityRef int,
	fieldNameRefs []int,
) ([]byte, error) {
	if len(fieldNameRefs) != len(point.Fields) {
		return nil, fmt.Errorf("field name refs count %d does not match field count %d", len(fieldNameRefs), len(point.Fields))
	}
	dst = binary.AppendUvarint(dst, uint64(identityRef))
	dst = binary.AppendUvarint(dst, point.SeriesID)
	dst = binary.AppendVarint(dst, point.Timestamp)
	dst = binary.AppendUvarint(dst, point.WriteSeq)
	dst = binary.AppendUvarint(dst, uint64(len(point.Fields)))
	for index, field := range point.Fields {
		var err error
		dst, err = appendField(dst, field, fieldNameRefs[index])
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func readPoint(
	reader *batchReader,
	identities []batchIdentity,
	fieldNames []string,
) (model.ResolvedPoint, error) {
	identityRef, err := reader.intCount("identity ref")
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	if identityRef >= len(identities) {
		return model.ResolvedPoint{}, fmt.Errorf("decode wal identity ref %d out of range", identityRef)
	}
	identity := identities[identityRef]
	seriesID, timestamp, writeSeq, err := readPointHeader(reader)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	fields, err := readFields(reader, fieldNames)
	if err != nil {
		return model.ResolvedPoint{}, err
	}
	return model.ResolvedPoint{
		Database:        identity.database,
		RetentionPolicy: identity.retentionPolicy,
		Measurement:     identity.measurement,
		Tags:            cloneStringMap(identity.tags),
		SeriesID:        seriesID,
		Timestamp:       timestamp,
		WriteSeq:        writeSeq,
		Fields:          fields,
	}, nil
}

func readPointHeader(reader *batchReader) (uint64, int64, uint64, error) {
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

func appendField(dst []byte, field model.ResolvedField, fieldNameRef int) ([]byte, error) {
	if field.Type != field.Value.Type {
		return nil, fmt.Errorf("field %s type %d does not match value type %d", field.FieldName, field.Type, field.Value.Type)
	}
	dst = binary.AppendUvarint(dst, uint64(field.FieldID))
	dst = binary.AppendUvarint(dst, uint64(fieldNameRef))
	dst = append(dst, byte(field.Type))
	return appendValuePayload(dst, field.Value)
}

func readFields(reader *batchReader, fieldNames []string) ([]model.ResolvedField, error) {
	count, err := reader.intCount("field count")
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	fields := make([]model.ResolvedField, 0, count)
	for range count {
		field, err := readField(reader, fieldNames)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func readField(reader *batchReader, fieldNames []string) (model.ResolvedField, error) {
	id, err := reader.uvarint("field id")
	if err != nil {
		return model.ResolvedField{}, err
	}
	fieldID, err := uint32Value("field id", id)
	if err != nil {
		return model.ResolvedField{}, err
	}
	nameRef, err := reader.intCount("field name ref")
	if err != nil {
		return model.ResolvedField{}, err
	}
	if nameRef >= len(fieldNames) {
		return model.ResolvedField{}, fmt.Errorf("decode wal field name ref %d out of range", nameRef)
	}
	fieldType, err := reader.byte("field type")
	if err != nil {
		return model.ResolvedField{}, err
	}
	value, err := readValuePayload(model.FieldType(fieldType), reader)
	if err != nil {
		return model.ResolvedField{}, err
	}
	return model.ResolvedField{
		FieldID:   fieldID,
		FieldName: fieldNames[nameRef],
		Type:      model.FieldType(fieldType),
		Value:     value,
	}, nil
}

func appendTags(dst []byte, tags map[string]string) []byte {
	if len(tags) == 0 {
		return binary.AppendUvarint(dst, 0)
	}
	if len(tags) == 1 {
		dst = binary.AppendUvarint(dst, 1)
		for key, value := range tags {
			dst = codec.AppendString(dst, key)
			dst = codec.AppendString(dst, value)
			return dst
		}
	}
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

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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
