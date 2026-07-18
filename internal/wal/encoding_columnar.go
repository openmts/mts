package wal

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

// 列式 WAL payload（formatID=2）：
// identities | field_schema | row_count |
// identity_ref[n] | series_id[n] |
// timestamps delta-varint[n] | write_seq delta-uvarint[n] |
// per-field: presence bitmap + values

type columnarFieldSchema struct {
	fieldID uint32
	name    string
	typ     model.FieldType
}

func encodeBatchInto(dst []byte, records []model.ResolvedPoint) ([]byte, error) {
	return encodeColumnarPointsInto(dst, records)
}

func encodeTypedBatchInto(
	dst []byte,
	batch model.ResolvedTypedBatch,
	rows []int,
) ([]byte, error) {
	if err := validateResolvedTypedBatch(batch, rows); err != nil {
		return nil, err
	}
	return encodeColumnarTypedInto(dst, batch, rows)
}

func encodeColumnarPointsInto(dst []byte, records []model.ResolvedPoint) ([]byte, error) {
	identities, identityRefs := batchIdentities(records)
	schema, present, values, err := buildPointFieldColumns(records)
	if err != nil {
		return nil, err
	}
	size := estimateColumnarPointsSize(identities, schema, records)
	if cap(dst) < size {
		dst = make([]byte, 0, size)
	} else {
		dst = dst[:0]
	}
	dst = appendColumnarIdentities(dst, identities)
	dst = appendColumnarSchema(dst, schema)
	dst = binary.AppendUvarint(dst, uint64(len(records)))
	for _, ref := range identityRefs {
		dst = binary.AppendUvarint(dst, uint64(ref))
	}
	for _, record := range records {
		dst = binary.AppendUvarint(dst, record.SeriesID)
	}
	dst = appendDeltaVarints(dst, pointTimestamps(records))
	dst = appendDeltaUvarints(dst, pointWriteSeqs(records))
	for fieldIndex := range schema {
		var encErr error
		dst, encErr = appendPresenceAndAlignedValues(dst, present[fieldIndex], values[fieldIndex])
		if encErr != nil {
			return nil, encErr
		}
	}
	return dst, nil
}

func encodeColumnarTypedInto(
	dst []byte,
	batch model.ResolvedTypedBatch,
	rows []int,
) ([]byte, error) {
	identityRows, identityRefs := typedBatchIdentities(batch, rows)
	schema := make([]columnarFieldSchema, len(batch.Fields))
	for index, field := range batch.Fields {
		schema[index] = columnarFieldSchema{
			fieldID: field.FieldID,
			name:    field.Name,
			typ:     field.Type,
		}
	}
	rowCount := typedRowCount(batch, rows)
	size := estimateColumnarTypedSize(batch, rows, identityRows, schema)
	if cap(dst) < size {
		dst = make([]byte, 0, size)
	} else {
		dst = dst[:0]
	}
	dst = binary.AppendUvarint(dst, uint64(len(identityRows)))
	tagOrder := typedTagOrder(batch.Tags)
	for _, row := range identityRows {
		dst = appendTypedIdentity(dst, batch, row, tagOrder)
	}
	dst = appendColumnarSchema(dst, schema)
	dst = binary.AppendUvarint(dst, uint64(rowCount))
	for _, ref := range identityRefs {
		dst = binary.AppendUvarint(dst, uint64(ref))
	}
	timestamps := make([]int64, rowCount)
	writeSeqs := make([]uint64, rowCount)
	for position := range rowCount {
		row := typedRowIndex(rows, position)
		dst = binary.AppendUvarint(dst, batch.SeriesIDs[row])
		timestamps[position] = batch.Timestamps[row]
		writeSeqs[position] = batch.WriteSeqs[row]
	}
	dst = appendDeltaVarints(dst, timestamps)
	dst = appendDeltaUvarints(dst, writeSeqs)
	for _, field := range batch.Fields {
		presence := make([]bool, rowCount)
		values := make([]model.FieldValue, rowCount)
		for position := range rowCount {
			row := typedRowIndex(rows, position)
			presence[position] = true
			value, err := typedFieldValue(field, row)
			if err != nil {
				return nil, err
			}
			values[position] = value
		}
		var encErr error
		dst, encErr = appendPresenceAndAlignedValues(dst, presence, values)
		if encErr != nil {
			return nil, encErr
		}
	}
	return dst, nil
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
	schema, err := readColumnarSchema(reader)
	if err != nil {
		return nil, err
	}
	rowCount, err := reader.intCount("row count")
	if err != nil {
		return nil, err
	}
	identityRefs := make([]int, rowCount)
	for index := range identityRefs {
		ref, err := reader.intCount("identity ref")
		if err != nil {
			return nil, err
		}
		if ref >= len(identities) {
			return nil, fmt.Errorf("decode wal identity ref %d out of range", ref)
		}
		identityRefs[index] = ref
	}
	seriesIDs := make([]uint64, rowCount)
	for index := range seriesIDs {
		seriesID, err := reader.uvarint("series id")
		if err != nil {
			return nil, err
		}
		seriesIDs[index] = seriesID
	}
	timestamps, err := readDeltaVarints(reader, rowCount)
	if err != nil {
		return nil, err
	}
	writeSeqs, err := readDeltaUvarints(reader, rowCount)
	if err != nil {
		return nil, err
	}
	fieldValues := make([][]model.FieldValue, len(schema))
	fieldPresent := make([][]bool, len(schema))
	for fieldIndex, field := range schema {
		presence, err := readPresenceBits(reader, rowCount)
		if err != nil {
			return nil, err
		}
		fieldPresent[fieldIndex] = presence
		values := make([]model.FieldValue, 0, rowCount)
		for row := 0; row < rowCount; row++ {
			if !presence[row] {
				continue
			}
			value, err := readValuePayload(field.typ, reader)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		fieldValues[fieldIndex] = values
	}
	if err := reader.done("wal columnar batch"); err != nil {
		return nil, err
	}
	records := make([]model.ResolvedPoint, rowCount)
	valueCursors := make([]int, len(schema))
	for row := 0; row < rowCount; row++ {
		identity := identities[identityRefs[row]]
		fields := make([]model.ResolvedField, 0, len(schema))
		for fieldIndex, field := range schema {
			if !fieldPresent[fieldIndex][row] {
				continue
			}
			cursor := valueCursors[fieldIndex]
			value := fieldValues[fieldIndex][cursor]
			valueCursors[fieldIndex] = cursor + 1
			fields = append(fields, model.ResolvedField{
				FieldID:   field.fieldID,
				FieldName: field.name,
				Type:      field.typ,
				Value:     value,
			})
		}
		records[row] = model.ResolvedPoint{
			Database:        identity.database,
			RetentionPolicy: identity.retentionPolicy,
			Measurement:     identity.measurement,
			Tags:            cloneStringMap(identity.tags),
			SeriesID:        seriesIDs[row],
			Timestamp:       timestamps[row],
			WriteSeq:        writeSeqs[row],
			Fields:          fields,
		}
	}
	return records, nil
}
