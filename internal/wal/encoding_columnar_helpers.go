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

func buildPointFieldSchema(records []model.ResolvedPoint) ([]columnarFieldSchema, error) {
	type key struct {
		id   uint32
		name string
		typ  model.FieldType
	}
	order := make([]key, 0)
	seen := make(map[key]struct{}, 8)
	for _, record := range records {
		for _, field := range record.Fields {
			if field.Type != field.Value.Type {
				return nil, fmt.Errorf(
					"field %s type %d does not match value type %d",
					field.FieldName,
					field.Type,
					field.Value.Type,
				)
			}
			item := key{id: field.FieldID, name: field.FieldName, typ: field.Type}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
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
	schema := make([]columnarFieldSchema, len(order))
	for index, item := range order {
		schema[index] = columnarFieldSchema{
			fieldID: item.id,
			name:    item.name,
			typ:     item.typ,
		}
	}
	return schema, nil
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
	size += len(records) * 20
	for range schema {
		size += (len(records)+7)/8 + len(records)*10
	}
	return size + 64
}

func estimateColumnarTypedSize(
	batch model.ResolvedTypedBatch,
	rows []int,
	identityRows []int,
	schema []columnarFieldSchema,
) int {
	size := uvarintSize(uint64(len(identityRows)))
	for _, row := range identityRows {
		size += estimateTypedIdentitySize(batch, row)
	}
	size += uvarintSize(uint64(len(schema)))
	for _, field := range schema {
		size += stringSize(field.name) + uvarintSize(uint64(field.fieldID)) + 1
	}
	rowCount := typedRowCount(batch, rows)
	size += uvarintSize(uint64(rowCount))
	size += rowCount * 20
	for _, field := range batch.Fields {
		size += (rowCount+7)/8 + estimateTypedColumnValuesSize(field, rows, rowCount)
	}
	return size + 64
}

func estimateTypedColumnValuesSize(
	field model.ResolvedTypedFieldColumn,
	rows []int,
	rowCount int,
) int {
	switch field.Type {
	case model.FieldFloat64:
		return rowCount * 8
	case model.FieldInt64:
		size := 0
		for position := range rowCount {
			row := typedRowIndex(rows, position)
			size += varintSize(field.Int64Values[row])
		}
		return size
	case model.FieldString:
		size := 0
		for position := range rowCount {
			row := typedRowIndex(rows, position)
			size += stringSize(field.StringValues[row])
		}
		return size
	case model.FieldBool:
		return rowCount
	default:
		return rowCount * 8
	}
}
