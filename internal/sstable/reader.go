package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/codec"
)

type blockReader struct {
	rest []byte
}

func newBlockReader(data []byte) *blockReader {
	return &blockReader{rest: data}
}

func (r *blockReader) byte(name string) (byte, error) {
	if len(r.rest) == 0 {
		return 0, fmt.Errorf("decode sstable %s: missing byte", name)
	}
	value := r.rest[0]
	r.rest = r.rest[1:]
	return value, nil
}

func (r *blockReader) uvarint(name string) (uint64, error) {
	value, size := binary.Uvarint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode sstable %s: invalid uvarint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}

func (r *blockReader) varint(name string) (int64, error) {
	value, size := binary.Varint(r.rest)
	if size <= 0 {
		return 0, fmt.Errorf("decode sstable %s: invalid varint", name)
	}
	r.rest = r.rest[size:]
	return value, nil
}

func (r *blockReader) uint32(name string) (uint32, error) {
	value, err := r.uvarint(name)
	if err != nil {
		return 0, err
	}
	if value > uint64(^uint32(0)) {
		return 0, fmt.Errorf("decode sstable %s: value %d overflows uint32", name, value)
	}
	return uint32(value), nil
}

func (r *blockReader) intCount(name string) (int, error) {
	value, err := r.uvarint(name)
	if err != nil {
		return 0, err
	}
	maxInt := uint64(int(^uint(0) >> 1))
	if value > maxInt {
		return 0, fmt.Errorf("decode sstable %s: count %d overflows int", name, value)
	}
	return int(value), nil
}

func (r *blockReader) fixedInt64(name string) (int64, error) {
	if len(r.rest) < 8 {
		return 0, fmt.Errorf("decode sstable %s: truncated int64", name)
	}
	value := int64(binary.LittleEndian.Uint64(r.rest[:8]))
	r.rest = r.rest[8:]
	return value, nil
}

func (r *blockReader) float64() (float64, error) {
	if len(r.rest) < 8 {
		return 0, fmt.Errorf("decode sstable float64 value: truncated payload")
	}
	value := math.Float64frombits(binary.LittleEndian.Uint64(r.rest[:8]))
	r.rest = r.rest[8:]
	return value, nil
}

func (r *blockReader) string(name string) (string, error) {
	value, rest, err := codec.ReadString(r.rest)
	if err != nil {
		return "", fmt.Errorf("decode sstable %s: %w", name, err)
	}
	r.rest = rest
	return value, nil
}

func (r *blockReader) done(name string) error {
	if len(r.rest) != 0 {
		return fmt.Errorf("decode %s: %d trailing bytes", name, len(r.rest))
	}
	return nil
}
