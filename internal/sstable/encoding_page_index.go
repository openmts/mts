package sstable

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/openmts/mts/internal/model"
)

func marshalValuePageIndex(dst []byte, index valuePageIndex) ([]byte, error) {
	dst = append(dst, valueEncodingPageIndex)
	dst = binary.AppendUvarint(dst, uint64(index.FieldID))
	dst = append(dst, byte(index.FieldType))
	dst = binary.AppendUvarint(dst, uint64(index.Count))
	dst = binary.AppendUvarint(dst, uint64(len(index.Pages)))
	for _, page := range index.Pages {
		dst = binary.AppendVarint(dst, page.MinTime)
		dst = binary.AppendVarint(dst, page.MaxTime)
		var err error
		dst, err = appendBlockRef(dst, page.Ref)
		if err != nil {
			return nil, err
		}
		dst = appendValuePageStats(dst, index.FieldType, page.Stats)
	}
	return dst, nil
}

func appendValuePageStats(
	dst []byte,
	fieldType model.FieldType,
	stats valuePageStats,
) []byte {
	if !stats.HasNumeric {
		return append(dst, 0)
	}
	dst = append(dst, valuePageStatsNumeric)
	switch fieldType {
	case model.FieldFloat64:
		dst = binary.LittleEndian.AppendUint64(dst, math.Float64bits(stats.MinFloat64))
		dst = binary.LittleEndian.AppendUint64(dst, math.Float64bits(stats.MaxFloat64))
	case model.FieldInt64:
		dst = binary.AppendVarint(dst, stats.MinInt64)
		dst = binary.AppendVarint(dst, stats.MaxInt64)
	default:
		dst[len(dst)-1] = 0
	}
	return dst
}

type valuePageIndexHeader struct {
	fieldID   uint32
	fieldType model.FieldType
	count     int
	pageCount int
}

var valuePageRefReadHook func()

func unmarshalValuePageIndex(payload []byte) (valuePageIndex, error) {
	reader := newBlockReader(payload)
	header, err := readValuePageIndexHeader(reader)
	if err != nil {
		return valuePageIndex{}, err
	}
	pages := make([]valuePageRef, 0, header.pageCount)
	for range header.pageCount {
		page, err := readValuePageRef(reader, header.fieldType)
		if err != nil {
			return valuePageIndex{}, err
		}
		pages = append(pages, page)
	}
	if err := reader.done("value page index"); err != nil {
		return valuePageIndex{}, err
	}
	return valuePageIndex{
		FieldID:   header.fieldID,
		FieldType: header.fieldType,
		Count:     header.count,
		Pages:     pages,
	}, nil
}

func readValuePageIndexHeader(reader *blockReader) (valuePageIndexHeader, error) {
	encoding, err := reader.byte("value page index encoding")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	if encoding != valueEncodingPageIndex {
		return valuePageIndexHeader{}, fmt.Errorf("unknown value page index encoding %d", encoding)
	}
	fieldID64, err := reader.uvarint("value page index field id")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	fieldID, err := uint32Value("value page index field id", fieldID64)
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	fieldType, err := reader.byte("value page index field type")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	count, err := reader.intCount("value page index sample count")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	pageCount, err := reader.intCount("value page index page count")
	if err != nil {
		return valuePageIndexHeader{}, err
	}
	return valuePageIndexHeader{
		fieldID:   fieldID,
		fieldType: model.FieldType(fieldType),
		count:     count,
		pageCount: pageCount,
	}, nil
}

func readValuePageRef(reader *blockReader, fieldType model.FieldType) (valuePageRef, error) {
	if valuePageRefReadHook != nil {
		valuePageRefReadHook()
	}
	minTime, err := reader.varint("value page min time")
	if err != nil {
		return valuePageRef{}, err
	}
	maxTime, err := reader.varint("value page max time")
	if err != nil {
		return valuePageRef{}, err
	}
	ref, err := readBlockRef(reader)
	if err != nil {
		return valuePageRef{}, err
	}
	stats, err := readValuePageStats(reader, fieldType)
	if err != nil {
		return valuePageRef{}, err
	}
	return valuePageRef{MinTime: minTime, MaxTime: maxTime, Ref: ref, Stats: stats}, nil
}

func readValuePageStats(reader *blockReader, fieldType model.FieldType) (valuePageStats, error) {
	flags, err := reader.byte("value page stats flags")
	if err != nil {
		return valuePageStats{}, err
	}
	if flags&valuePageStatsNumeric == 0 {
		return valuePageStats{}, nil
	}
	stats := valuePageStats{HasNumeric: true}
	switch fieldType {
	case model.FieldFloat64:
		minValue, err := reader.float64()
		if err != nil {
			return valuePageStats{}, err
		}
		maxValue, err := reader.float64()
		if err != nil {
			return valuePageStats{}, err
		}
		stats.MinFloat64 = minValue
		stats.MaxFloat64 = maxValue
	case model.FieldInt64:
		minValue, err := reader.varint("value page min int")
		if err != nil {
			return valuePageStats{}, err
		}
		maxValue, err := reader.varint("value page max int")
		if err != nil {
			return valuePageStats{}, err
		}
		stats.MinInt64 = minValue
		stats.MaxInt64 = maxValue
	default:
		return valuePageStats{}, fmt.Errorf("numeric stats unsupported for field type %d", fieldType)
	}
	return stats, nil
}

func uint32Value(name string, value uint64) (uint32, error) {
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("%s overflows uint32", name)
	}
	return uint32(value), nil
}
