package wal

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

func appendColumnarIdentities(dst []byte, identities []batchIdentity) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(identities)))
	for _, identity := range identities {
		dst = appendIdentity(dst, identity)
	}
	return dst
}

func appendColumnarSchema(dst []byte, schema []columnarFieldSchema) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(schema)))
	for _, field := range schema {
		dst = codec.AppendString(dst, field.name)
		dst = binary.AppendUvarint(dst, uint64(field.fieldID))
		dst = append(dst, byte(field.typ))
	}
	return dst
}

func readColumnarSchema(reader *batchReader) ([]columnarFieldSchema, error) {
	count, err := reader.intCount("field schema count")
	if err != nil {
		return nil, err
	}
	schema := make([]columnarFieldSchema, count)
	for index := range schema {
		name, err := reader.string("field name")
		if err != nil {
			return nil, err
		}
		fieldID, err := reader.uvarint("field id")
		if err != nil {
			return nil, err
		}
		typ, err := reader.byte("field type")
		if err != nil {
			return nil, err
		}
		schema[index] = columnarFieldSchema{
			fieldID: uint32(fieldID),
			name:    name,
			typ:     model.FieldType(typ),
		}
	}
	return schema, nil
}

func buildPointFieldColumns(
	records []model.ResolvedPoint,
) ([]columnarFieldSchema, [][]bool, [][]model.FieldValue, error) {
	type key struct {
		id   uint32
		name string
		typ  model.FieldType
	}
	order := make([]key, 0)
	seen := make(map[key]int)
	for _, record := range records {
		for _, field := range record.Fields {
			if field.Type != field.Value.Type {
				return nil, nil, nil, fmt.Errorf(
					"field %s type %d does not match value type %d",
					field.FieldName, field.Type, field.Value.Type,
				)
			}
			item := key{id: field.FieldID, name: field.FieldName, typ: field.Type}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = len(order)
			order = append(order, item)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].id != order[j].id {
			return order[i].id < order[j].id
		}
		if order[i].name != order[j].name {
			return order[i].name < order[j].name
		}
		return order[i].typ < order[j].typ
	})
	for index, item := range order {
		seen[item] = index
	}
	schema := make([]columnarFieldSchema, len(order))
	present := make([][]bool, len(order))
	values := make([][]model.FieldValue, len(order))
	for index, item := range order {
		schema[index] = columnarFieldSchema{fieldID: item.id, name: item.name, typ: item.typ}
		present[index] = make([]bool, len(records))
		values[index] = make([]model.FieldValue, len(records))
	}
	for row, record := range records {
		for _, field := range record.Fields {
			item := key{id: field.FieldID, name: field.FieldName, typ: field.Type}
			fieldIndex := seen[item]
			present[fieldIndex][row] = true
			values[fieldIndex][row] = field.Value
		}
	}
	return schema, present, values, nil
}

func appendPresenceAndAlignedValues(
	dst []byte,
	presence []bool,
	column []model.FieldValue,
) ([]byte, error) {
	dst = appendPresenceBits(dst, presence)
	for index, present := range presence {
		if !present {
			continue
		}
		var err error
		dst, err = appendValuePayload(dst, column[index])
		if err != nil {
			return nil, err
		}
	}
	return dst, nil
}

func appendPresenceBits(dst []byte, presence []bool) []byte {
	byteCount := (len(presence) + 7) / 8
	if byteCount == 0 {
		return dst
	}
	start := len(dst)
	for cap(dst)-len(dst) < byteCount {
		// grow
		dst = append(dst, 0)
	}
	dst = dst[:start+byteCount]
	bits := dst[start:]
	clear(bits)
	for index, present := range presence {
		if present {
			bits[index/8] |= 1 << uint(index%8)
		}
	}
	return dst
}

func readPresenceBits(reader *batchReader, rowCount int) ([]bool, error) {
	byteCount := (rowCount + 7) / 8
	raw, err := reader.rawBytes(byteCount, "presence bitmap")
	if err != nil {
		return nil, err
	}
	presence := make([]bool, rowCount)
	for index := 0; index < rowCount; index++ {
		if raw[index/8]&(1<<uint(index%8)) != 0 {
			presence[index] = true
		}
	}
	return presence, nil
}

func appendDeltaVarints(dst []byte, values []int64) []byte {
	var prev int64
	for index, value := range values {
		delta := value
		if index > 0 {
			delta = value - prev
		}
		dst = binary.AppendVarint(dst, delta)
		prev = value
	}
	return dst
}

func appendDeltaUvarints(dst []byte, values []uint64) []byte {
	var prev uint64
	for index, value := range values {
		if index == 0 {
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

func readDeltaVarints(reader *batchReader, count int) ([]int64, error) {
	out := make([]int64, count)
	var prev int64
	for index := 0; index < count; index++ {
		delta, err := reader.varint("timestamp delta")
		if err != nil {
			return nil, err
		}
		if index == 0 {
			out[index] = delta
		} else {
			out[index] = prev + delta
		}
		prev = out[index]
	}
	return out, nil
}

func readDeltaUvarints(reader *batchReader, count int) ([]uint64, error) {
	out := make([]uint64, count)
	var prev uint64
	for index := 0; index < count; index++ {
		token, err := reader.uvarint("write seq token")
		if err != nil {
			return nil, err
		}
		if index == 0 {
			out[index] = token
			prev = token
			continue
		}
		if token == 0 {
			absolute, err := reader.uvarint("write seq absolute")
			if err != nil {
				return nil, err
			}
			out[index] = absolute
			prev = absolute
			continue
		}
		out[index] = prev + token - 1
		prev = out[index]
	}
	return out, nil
}

func pointTimestamps(records []model.ResolvedPoint) []int64 {
	out := make([]int64, len(records))
	for index, record := range records {
		out[index] = record.Timestamp
	}
	return out
}

func pointWriteSeqs(records []model.ResolvedPoint) []uint64 {
	out := make([]uint64, len(records))
	for index, record := range records {
		out[index] = record.WriteSeq
	}
	return out
}

func typedFieldValue(field model.ResolvedTypedFieldColumn, row int) (model.FieldValue, error) {
	switch field.Type {
	case model.FieldFloat64:
		return model.Float64Value(field.Float64Values[row]), nil
	case model.FieldInt64:
		return model.Int64Value(field.Int64Values[row]), nil
	case model.FieldString:
		return model.StringValue(field.StringValues[row]), nil
	case model.FieldBool:
		return model.BoolValue(field.BoolValues[row]), nil
	default:
		return model.FieldValue{}, fmt.Errorf("unsupported typed field value type %d", field.Type)
	}
}

func estimateColumnarPointsSize(
	identities []batchIdentity,
	schema []columnarFieldSchema,
	records []model.ResolvedPoint,
) int {
	size := uvarintSize(uint64(len(identities)))
	for _, identity := range identities {
		size += stringSize(identity.database)
		size += stringSize(identity.retentionPolicy)
		size += stringSize(identity.measurement)
		size += estimateTagsSize(identity.tags)
	}
	size += uvarintSize(uint64(len(schema)))
	for _, field := range schema {
		size += stringSize(field.name) + uvarintSize(uint64(field.fieldID)) + 1
	}
	size += uvarintSize(uint64(len(records)))
	size += len(records) * 24
	size += len(schema) * ((len(records)+7)/8 + 16)
	return size*2 + 256
}

func estimateColumnarTypedSize(
	batch model.ResolvedTypedBatch,
	rows []int,
	identityRows []int,
	schema []columnarFieldSchema,
) int {
	size := estimateTypedBatchSize(batch, rows, identityRows)
	rowCount := typedRowCount(batch, rows)
	size += len(schema) * ((rowCount+7)/8 + 8)
	return size*2 + 256
}
