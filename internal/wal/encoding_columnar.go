package wal

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/codec"
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
	schema, err := buildPointFieldSchema(records)
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
	dst = appendPointTimestampDeltas(dst, records)
	dst = appendPointWriteSeqDeltas(dst, records)
	var encErr error
	dst, encErr = appendPointFieldColumns(dst, records, schema)
	if encErr != nil {
		return nil, encErr
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
	for position := range rowCount {
		row := typedRowIndex(rows, position)
		dst = binary.AppendUvarint(dst, batch.SeriesIDs[row])
	}
	dst = appendTypedTimestampDeltas(dst, batch, rows, rowCount)
	dst = appendTypedWriteSeqDeltas(dst, batch, rows, rowCount)
	for _, field := range batch.Fields {
		var encErr error
		dst, encErr = appendTypedFieldColumnDense(dst, field, rows, rowCount)
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

func appendPointTimestampDeltas(dst []byte, records []model.ResolvedPoint) []byte {
	var prev int64
	for index, record := range records {
		delta := record.Timestamp
		if index > 0 {
			delta = record.Timestamp - prev
		}
		dst = binary.AppendVarint(dst, delta)
		prev = record.Timestamp
	}
	return dst
}

func appendPointWriteSeqDeltas(dst []byte, records []model.ResolvedPoint) []byte {
	var prev uint64
	for index, record := range records {
		if index == 0 {
			dst = binary.AppendUvarint(dst, record.WriteSeq)
			prev = record.WriteSeq
			continue
		}
		if record.WriteSeq < prev {
			dst = binary.AppendUvarint(dst, 0)
			dst = binary.AppendUvarint(dst, record.WriteSeq)
			prev = record.WriteSeq
			continue
		}
		dst = binary.AppendUvarint(dst, record.WriteSeq-prev+1)
		prev = record.WriteSeq
	}
	return dst
}

func appendTypedTimestampDeltas(
	dst []byte,
	batch model.ResolvedTypedBatch,
	rows []int,
	rowCount int,
) []byte {
	var prev int64
	for position := range rowCount {
		row := typedRowIndex(rows, position)
		value := batch.Timestamps[row]
		delta := value
		if position > 0 {
			delta = value - prev
		}
		dst = binary.AppendVarint(dst, delta)
		prev = value
	}
	return dst
}

func appendTypedWriteSeqDeltas(
	dst []byte,
	batch model.ResolvedTypedBatch,
	rows []int,
	rowCount int,
) []byte {
	var prev uint64
	for position := range rowCount {
		row := typedRowIndex(rows, position)
		value := batch.WriteSeqs[row]
		if position == 0 {
			dst = binary.AppendUvarint(dst, value)
			prev = value
			continue
		}
		if value < prev {
			dst = binary.AppendUvarint(dst, 0)
			dst = binary.AppendUvarint(dst, value)
			prev = value
			continue
		}
		dst = binary.AppendUvarint(dst, value-prev+1)
		prev = value
	}
	return dst
}

func appendPointFieldColumns(
	dst []byte,
	records []model.ResolvedPoint,
	schema []columnarFieldSchema,
) ([]byte, error) {
	if len(schema) == 0 {
		return dst, nil
	}
	// 同构宽表（常见）：按 schema 顺序直接写出，避免 rows×fields 二次查找。
	if pointFieldsMatchSchema(records, schema) {
		return appendPointFieldColumnsDense(dst, records, schema)
	}
	for _, field := range schema {
		var err error
		dst, err = appendPointFieldColumn(dst, records, field)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func pointFieldsMatchSchema(records []model.ResolvedPoint, schema []columnarFieldSchema) bool {
	if len(records) == 0 {
		return true
	}
	// 抽样检查前/中/后三点，失败则回退通用路径。
	sample := []int{0, len(records) / 2, len(records) - 1}
	for _, index := range sample {
		if index < 0 || index >= len(records) {
			continue
		}
		fields := records[index].Fields
		if len(fields) != len(schema) {
			return false
		}
		for fieldIndex, field := range schema {
			item := fields[fieldIndex]
			if item.FieldID != field.fieldID || item.FieldName != field.name || item.Type != field.typ {
				return false
			}
		}
	}
	return true
}

func appendPointFieldColumnsDense(
	dst []byte,
	records []model.ResolvedPoint,
	schema []columnarFieldSchema,
) ([]byte, error) {
	rowCount := len(records)
	byteCount := (rowCount + 7) / 8
	for fieldIndex := range schema {
		start := len(dst)
		dst = growBytes(dst, byteCount)
		bits := dst[start : start+byteCount]
		// dense presence
		for index := range bits {
			bits[index] = 0xff
		}
		if rem := rowCount % 8; rem != 0 && byteCount > 0 {
			bits[byteCount-1] = byte((1 << rem) - 1)
		}
		for _, record := range records {
			if fieldIndex >= len(record.Fields) {
				return nil, fmt.Errorf("dense field index %d out of range", fieldIndex)
			}
			var err error
			dst, err = appendValuePayload(dst, record.Fields[fieldIndex].Value)
			if err != nil {
				return nil, err
			}
		}
	}
	return dst, nil
}

func appendPointFieldColumn(
	dst []byte,
	records []model.ResolvedPoint,
	field columnarFieldSchema,
) ([]byte, error) {
	byteCount := (len(records) + 7) / 8
	start := len(dst)
	dst = growBytes(dst, byteCount)
	bits := dst[start : start+byteCount]
	clear(bits)
	for row, record := range records {
		value, ok := lookupPointField(record, field)
		if !ok {
			continue
		}
		bits[row/8] |= 1 << uint(row%8)
		var err error
		dst, err = appendValuePayload(dst, value)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func lookupPointField(
	record model.ResolvedPoint,
	field columnarFieldSchema,
) (model.FieldValue, bool) {
	for _, item := range record.Fields {
		if item.FieldID == field.fieldID &&
			item.FieldName == field.name &&
			item.Type == field.typ {
			return item.Value, true
		}
	}
	return model.FieldValue{}, false
}

func appendTypedFieldColumnDense(
	dst []byte,
	field model.ResolvedTypedFieldColumn,
	rows []int,
	rowCount int,
) ([]byte, error) {
	byteCount := (rowCount + 7) / 8
	start := len(dst)
	dst = growBytes(dst, byteCount)
	bits := dst[start : start+byteCount]
	for index := range bits {
		bits[index] = 0xff
	}
	// 清理尾部多余 bit。
	if rem := rowCount % 8; rem != 0 && byteCount > 0 {
		bits[byteCount-1] = byte((1 << rem) - 1)
	}
	for position := range rowCount {
		row := typedRowIndex(rows, position)
		var err error
		dst, err = appendTypedScalar(dst, field, row)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func appendTypedScalar(
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

func growBytes(dst []byte, n int) []byte {
	if n <= 0 {
		return dst
	}
	start := len(dst)
	if cap(dst)-len(dst) < n {
		grown := make([]byte, start+n)
		copy(grown, dst)
		dst = grown
	} else {
		dst = dst[:start+n]
	}
	return dst
}
