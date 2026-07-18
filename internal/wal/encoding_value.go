package wal

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

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
