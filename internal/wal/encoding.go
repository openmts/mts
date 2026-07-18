package wal

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/collections"
	"github.com/openmts/mts/internal/model"
)

func encodeBatch(records []model.ResolvedPoint) ([]byte, error) {
	return encodeBatchInto(nil, records)
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
	identities := make([]batchIdentity, 0, min(len(records), 16))
	refs := make([]int, len(records))
	seenSeries := newSeriesRefIndex(records)
	seenIdentity := make(map[string]int)
	keyScratch := make([]byte, 0, 64)
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

func appendField(dst []byte, field model.ResolvedField, fieldNameRef int) ([]byte, error) {
	if field.Type != field.Value.Type {
		return nil, fmt.Errorf("field %s type %d does not match value type %d", field.FieldName, field.Type, field.Value.Type)
	}
	dst = binary.AppendUvarint(dst, uint64(field.FieldID))
	dst = binary.AppendUvarint(dst, uint64(fieldNameRef))
	dst = append(dst, byte(field.Type))
	return appendValuePayload(dst, field.Value)
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
	return collections.CloneMapNilIfEmpty(values)
}
