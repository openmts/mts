package codec

import (
	"encoding/binary"
	"fmt"
	"math"

	"codeberg.org/mts/mts/internal/model"
)

func AppendString(dst []byte, value string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func ReadString(src []byte) (string, []byte, error) {
	length, size := binary.Uvarint(src)
	if size <= 0 {
		return "", nil, fmt.Errorf("read string: invalid length")
	}
	start := size
	if length > uint64(len(src)-start) {
		return "", nil, fmt.Errorf("read string: truncated payload")
	}
	end := start + int(length)
	return string(src[start:end]), src[end:], nil
}

func AppendFieldValue(dst []byte, value model.FieldValue) []byte {
	dst = append(dst, byte(value.Type))
	switch value.Type {
	case model.FieldFloat64:
		return binary.LittleEndian.AppendUint64(dst, math.Float64bits(value.Float64))
	case model.FieldInt64:
		return binary.AppendVarint(dst, value.Int64)
	case model.FieldString:
		return AppendString(dst, value.String)
	case model.FieldBool:
		if value.Bool {
			return append(dst, 1)
		}
		return append(dst, 0)
	default:
		return dst
	}
}

func ReadFieldValue(src []byte) (model.FieldValue, []byte, error) {
	if len(src) == 0 {
		return model.FieldValue{}, nil, fmt.Errorf("read field value: missing type")
	}
	fieldType := model.FieldType(src[0])
	return readTypedFieldValue(fieldType, src[1:])
}

func AppendBoolBits(dst []byte, values []bool) []byte {
	byteCount := (len(values) + 7) / 8
	start := len(dst)
	dst = append(dst, make([]byte, byteCount)...)
	for index, value := range values {
		if value {
			dst[start+index/8] |= 1 << uint(index%8)
		}
	}
	return dst
}

func ReadBoolBits(src []byte, count int) ([]bool, []byte, error) {
	if count < 0 {
		return nil, nil, fmt.Errorf("read bool bits: negative count %d", count)
	}
	byteCount := (count + 7) / 8
	if len(src) < byteCount {
		return nil, nil, fmt.Errorf("read bool bits: truncated payload")
	}
	values := make([]bool, count)
	for index := range count {
		values[index] = src[index/8]&(1<<uint(index%8)) != 0
	}
	return values, src[byteCount:], nil
}

func readTypedFieldValue(fieldType model.FieldType, src []byte) (model.FieldValue, []byte, error) {
	switch fieldType {
	case model.FieldFloat64:
		return readFloat64Value(src)
	case model.FieldInt64:
		return readInt64Value(src)
	case model.FieldString:
		value, rest, err := ReadString(src)
		return model.StringValue(value), rest, err
	case model.FieldBool:
		return readBoolValue(src)
	default:
		return model.FieldValue{}, nil, fmt.Errorf("read field value: unsupported type %d", fieldType)
	}
}

func readFloat64Value(src []byte) (model.FieldValue, []byte, error) {
	if len(src) < 8 {
		return model.FieldValue{}, nil, fmt.Errorf("read float64 field value: truncated payload")
	}
	bits := binary.LittleEndian.Uint64(src[:8])
	return model.Float64Value(math.Float64frombits(bits)), src[8:], nil
}

func readInt64Value(src []byte) (model.FieldValue, []byte, error) {
	value, size := binary.Varint(src)
	if size <= 0 {
		return model.FieldValue{}, nil, fmt.Errorf("read int64 field value: invalid varint")
	}
	return model.Int64Value(value), src[size:], nil
}

func readBoolValue(src []byte) (model.FieldValue, []byte, error) {
	if len(src) == 0 {
		return model.FieldValue{}, nil, fmt.Errorf("read bool field value: truncated payload")
	}
	switch src[0] {
	case 0:
		return model.BoolValue(false), src[1:], nil
	case 1:
		return model.BoolValue(true), src[1:], nil
	default:
		return model.FieldValue{}, nil, fmt.Errorf("read bool field value: invalid byte %d", src[0])
	}
}
