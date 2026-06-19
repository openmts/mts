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
	return encodeBatchInto(nil, records)
}

func encodeBatchInto(dst []byte, records []model.ResolvedPoint) ([]byte, error) {
	identities, identityRefs := batchIdentities(records)
	fieldNames, fieldNameRefs := batchFieldNames(records)
	size := estimateBatchSize(records)
	if cap(dst) < size {
		dst = make([]byte, 0, size)
	} else {
		dst = dst[:0]
	}
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

func encodeTypedBatchInto(
	dst []byte,
	batch model.ResolvedTypedBatch,
	rows []int,
) ([]byte, error) {
	if err := validateResolvedTypedBatch(batch, rows); err != nil {
		return nil, err
	}
	identityRows, identityRefs := typedBatchIdentities(batch, rows)
	size := estimateTypedBatchSize(batch, rows, identityRows)
	if cap(dst) < size {
		dst = make([]byte, 0, size)
	} else {
		dst = dst[:0]
	}
	tagOrder := typedTagOrder(batch.Tags)
	dst = binary.AppendUvarint(dst, uint64(len(identityRows)))
	for _, row := range identityRows {
		dst = appendTypedIdentity(dst, batch, row, tagOrder)
	}
	dst = appendTypedFieldNames(dst, batch.Fields)
	dst = binary.AppendUvarint(dst, uint64(typedRowCount(batch, rows)))
	for position, ref := range identityRefs {
		row := typedRowIndex(rows, position)
		var err error
		dst, err = appendTypedPoint(dst, batch, row, ref)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func validateResolvedTypedBatch(batch model.ResolvedTypedBatch, rows []int) error {
	count := len(batch.Timestamps)
	if len(batch.SeriesIDs) != count || len(batch.WriteSeqs) != count {
		return fmt.Errorf("resolved typed batch row metadata length mismatch")
	}
	for _, row := range rows {
		if row < 0 || row >= count {
			return fmt.Errorf("resolved typed row index %d out of range", row)
		}
	}
	for _, field := range batch.Fields {
		if typedResolvedFieldLen(field) != count {
			return fmt.Errorf("resolved typed field %s length mismatch", field.Name)
		}
	}
	return nil
}

type batchIdentity struct {
	database        string
	retentionPolicy string
	measurement     string
	tags            map[string]string
}

const denseSeriesRefMaxSpanFactor = 4

func batchIdentities(records []model.ResolvedPoint) ([]batchIdentity, []int) {
	identities := make([]batchIdentity, 0)
	refs := make([]int, len(records))
	seenSeries := newSeriesRefIndex(records)
	seenIdentity := make(map[string]int)
	keyScratch := make([]byte, 0)
	for index, record := range records {
		if ref, ok := lastIdentityRef(record, identities); ok {
			refs[index] = ref
			continue
		}

		if record.SeriesID != 0 {
			if ref, ok := seenSeries.lookup(record.SeriesID); ok {
				if identityMatches(record, identities[ref]) {
					refs[index] = ref
					continue
				}
			} else {
				ref = len(identities)
				seenSeries.setIfAbsent(record.SeriesID, ref)
				identities = append(identities, newBatchIdentity(record))
				refs[index] = ref
				continue
			}
		}

		ref, scratch := identityRefByKey(record, seenIdentity, identities, keyScratch)
		keyScratch = scratch
		if ref == len(identities) {
			identities = append(identities, newBatchIdentity(record))
		}
		refs[index] = ref
		if record.SeriesID != 0 {
			seenSeries.setIfAbsent(record.SeriesID, ref)
		}
	}
	return identities, refs
}

type seriesRefIndex struct {
	sparse map[uint64]int
	base   uint64
	dense  []int
}

func newSeriesRefIndex(records []model.ResolvedPoint) seriesRefIndex {
	minID, maxID, ok := seriesIDRange(records)
	if !ok {
		return seriesRefIndex{}
	}
	span := maxID - minID + 1
	maxDenseSpan := uint64(len(records) * denseSeriesRefMaxSpanFactor)
	if span <= maxDenseSpan {
		return seriesRefIndex{base: minID, dense: make([]int, int(span))}
	}
	return seriesRefIndex{sparse: make(map[uint64]int, len(records))}
}

func seriesIDRange(records []model.ResolvedPoint) (uint64, uint64, bool) {
	var minID uint64
	var maxID uint64
	found := false
	for _, record := range records {
		if record.SeriesID == 0 {
			continue
		}
		if !found || record.SeriesID < minID {
			minID = record.SeriesID
		}
		if record.SeriesID > maxID {
			maxID = record.SeriesID
		}
		found = true
	}
	return minID, maxID, found
}

func (i seriesRefIndex) lookup(seriesID uint64) (int, bool) {
	if len(i.dense) > 0 {
		if seriesID < i.base || seriesID-i.base >= uint64(len(i.dense)) {
			return 0, false
		}
		ref := i.dense[seriesID-i.base]
		return ref - 1, ref != 0
	}
	ref, ok := i.sparse[seriesID]
	return ref, ok
}

func (i seriesRefIndex) setIfAbsent(seriesID uint64, ref int) {
	if len(i.dense) > 0 {
		if seriesID < i.base || seriesID-i.base >= uint64(len(i.dense)) {
			return
		}
		slot := &i.dense[seriesID-i.base]
		if *slot == 0 {
			*slot = ref + 1
		}
		return
	}
	if i.sparse == nil {
		return
	}
	if _, ok := i.sparse[seriesID]; !ok {
		i.sparse[seriesID] = ref
	}
}

func typedBatchIdentities(batch model.ResolvedTypedBatch, rows []int) ([]int, []int) {
	count := typedRowCount(batch, rows)
	identityRows := make([]int, 0)
	refs := make([]int, count)
	seen := newTypedSeriesRefIndex(batch, rows)
	for position := range count {
		row := typedRowIndex(rows, position)
		seriesID := batch.SeriesIDs[row]
		if ref, ok := seen.lookup(seriesID); ok {
			refs[position] = ref
			continue
		}
		ref := len(identityRows)
		seen.setIfAbsent(seriesID, ref)
		identityRows = append(identityRows, row)
		refs[position] = ref
	}
	return identityRows, refs
}

func newTypedSeriesRefIndex(batch model.ResolvedTypedBatch, rows []int) seriesRefIndex {
	minID, maxID, ok := typedSeriesIDRange(batch, rows)
	if !ok {
		return seriesRefIndex{}
	}
	span := maxID - minID + 1
	maxDenseSpan := uint64(typedRowCount(batch, rows) * denseSeriesRefMaxSpanFactor)
	if span <= maxDenseSpan {
		return seriesRefIndex{base: minID, dense: make([]int, int(span))}
	}
	return seriesRefIndex{sparse: make(map[uint64]int, typedRowCount(batch, rows))}
}

func typedSeriesIDRange(batch model.ResolvedTypedBatch, rows []int) (uint64, uint64, bool) {
	var minID uint64
	var maxID uint64
	found := false
	for position := range typedRowCount(batch, rows) {
		seriesID := batch.SeriesIDs[typedRowIndex(rows, position)]
		if seriesID == 0 {
			continue
		}
		if !found || seriesID < minID {
			minID = seriesID
		}
		if seriesID > maxID {
			maxID = seriesID
		}
		found = true
	}
	return minID, maxID, found
}

func identityRefByKey(
	record model.ResolvedPoint,
	seen map[string]int,
	identities []batchIdentity,
	scratch []byte,
) (int, []byte) {
	key, scratch := identityKeyWithScratch(record, scratch[:0])
	ref, ok := seen[key]
	if ok {
		return ref, scratch
	}
	ref = len(identities)
	seen[key] = ref
	return ref, scratch
}

func newBatchIdentity(record model.ResolvedPoint) batchIdentity {
	return batchIdentity{
		database:        record.Database,
		retentionPolicy: record.RetentionPolicy,
		measurement:     record.Measurement,
		tags:            record.Tags,
	}
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

func appendTypedIdentity(
	dst []byte,
	batch model.ResolvedTypedBatch,
	row int,
	tagOrder []int,
) []byte {
	dst = codec.AppendString(dst, batch.Database)
	dst = codec.AppendString(dst, batch.RetentionPolicy)
	dst = codec.AppendString(dst, batch.Measurement)
	return appendTypedTags(dst, batch.Tags, row, tagOrder)
}

func appendTypedFieldNames(dst []byte, fields []model.ResolvedTypedFieldColumn) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(fields)))
	for _, field := range fields {
		dst = codec.AppendString(dst, field.Name)
	}
	return dst
}

func estimateBatchSize(records []model.ResolvedPoint) int {
	size := uvarintSize(uint64(len(records)))
	for _, record := range records {
		size += estimatePointSize(record)
	}
	return size
}

func estimateTypedBatchSize(
	batch model.ResolvedTypedBatch,
	rows []int,
	identityRows []int,
) int {
	size := uvarintSize(uint64(len(identityRows)))
	for _, row := range identityRows {
		size += estimateTypedIdentitySize(batch, row)
	}
	size += uvarintSize(uint64(len(batch.Fields)))
	for _, field := range batch.Fields {
		size += stringSize(field.Name)
	}
	for position := range typedRowCount(batch, rows) {
		size += estimateTypedPointSize(batch, typedRowIndex(rows, position))
	}
	return size
}

func estimateTypedIdentitySize(batch model.ResolvedTypedBatch, row int) int {
	size := stringSize(batch.Database)
	size += stringSize(batch.RetentionPolicy)
	size += stringSize(batch.Measurement)
	for _, tag := range batch.Tags {
		size += stringSize(tag.Name) + stringSize(tag.Values[row])
	}
	return size + uvarintSize(uint64(len(batch.Tags)))
}

func estimateTypedPointSize(batch model.ResolvedTypedBatch, row int) int {
	size := uvarintSize(0) + uvarintSize(batch.SeriesIDs[row])
	size += varintSize(batch.Timestamps[row])
	size += uvarintSize(batch.WriteSeqs[row])
	size += uvarintSize(uint64(len(batch.Fields)))
	for _, field := range batch.Fields {
		size += estimateTypedFieldSize(field, row)
	}
	return size
}

func estimateTypedFieldSize(field model.ResolvedTypedFieldColumn, row int) int {
	size := uvarintSize(uint64(field.FieldID)) + uvarintSize(0) + 1
	switch field.Type {
	case model.FieldFloat64:
		return size + 8
	case model.FieldInt64:
		return size + varintSize(field.Int64Values[row])
	case model.FieldString:
		return size + stringSize(field.StringValues[row])
	case model.FieldBool:
		return size + 1
	default:
		return size
	}
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

func appendTypedPoint(
	dst []byte,
	batch model.ResolvedTypedBatch,
	row int,
	identityRef int,
) ([]byte, error) {
	dst = binary.AppendUvarint(dst, uint64(identityRef))
	dst = binary.AppendUvarint(dst, batch.SeriesIDs[row])
	dst = binary.AppendVarint(dst, batch.Timestamps[row])
	dst = binary.AppendUvarint(dst, batch.WriteSeqs[row])
	dst = binary.AppendUvarint(dst, uint64(len(batch.Fields)))
	for fieldNameRef, field := range batch.Fields {
		var err error
		dst, err = appendTypedField(dst, field, row, fieldNameRef)
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

func appendTypedField(
	dst []byte,
	field model.ResolvedTypedFieldColumn,
	row int,
	fieldNameRef int,
) ([]byte, error) {
	dst = binary.AppendUvarint(dst, uint64(field.FieldID))
	dst = binary.AppendUvarint(dst, uint64(fieldNameRef))
	dst = append(dst, byte(field.Type))
	return appendTypedValuePayload(dst, field, row)
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

func appendTypedTags(dst []byte, tags []model.TagColumn, row int, order []int) []byte {
	if len(tags) == 0 {
		return binary.AppendUvarint(dst, 0)
	}
	dst = binary.AppendUvarint(dst, uint64(len(tags)))
	if len(tags) == 1 {
		tag := tags[0]
		dst = codec.AppendString(dst, tag.Name)
		return codec.AppendString(dst, tag.Values[row])
	}
	for _, index := range order {
		tag := tags[index]
		dst = codec.AppendString(dst, tag.Name)
		dst = codec.AppendString(dst, tag.Values[row])
	}
	return dst
}

func typedTagOrder(tags []model.TagColumn) []int {
	if len(tags) <= 1 {
		return nil
	}
	order := make([]int, len(tags))
	for index := range tags {
		order[index] = index
	}
	sort.Slice(order, func(i int, j int) bool {
		return tags[order[i]].Name < tags[order[j]].Name
	})
	return order
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

func appendTypedValuePayload(
	dst []byte,
	field model.ResolvedTypedFieldColumn,
	row int,
) ([]byte, error) {
	switch field.Type {
	case model.FieldFloat64:
		return binary.LittleEndian.AppendUint64(dst, math.Float64bits(field.Float64Values[row])), nil
	case model.FieldInt64:
		return binary.AppendVarint(dst, field.Int64Values[row]), nil
	case model.FieldString:
		return codec.AppendString(dst, field.StringValues[row]), nil
	case model.FieldBool:
		if field.BoolValues[row] {
			return append(dst, 1), nil
		}
		return append(dst, 0), nil
	default:
		return nil, fmt.Errorf("unsupported typed field value type %d", field.Type)
	}
}

func typedResolvedFieldLen(field model.ResolvedTypedFieldColumn) int {
	switch field.Type {
	case model.FieldFloat64:
		return len(field.Float64Values)
	case model.FieldInt64:
		return len(field.Int64Values)
	case model.FieldString:
		return len(field.StringValues)
	case model.FieldBool:
		return len(field.BoolValues)
	default:
		return -1
	}
}

func typedRowCount(batch model.ResolvedTypedBatch, rows []int) int {
	if len(rows) > 0 {
		return len(rows)
	}
	return len(batch.Timestamps)
}

func typedRowIndex(rows []int, position int) int {
	if len(rows) > 0 {
		return rows[position]
	}
	return position
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
