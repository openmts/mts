package wal

import (
	"github.com/openmts/mts/internal/model"
)

func estimateTypedIdentitySize(batch model.ResolvedTypedBatch, row int) int {
	size := stringSize(batch.Database)
	size += stringSize(batch.RetentionPolicy)
	size += stringSize(batch.Measurement)
	for _, tag := range batch.Tags {
		size += stringSize(tag.Name) + stringSize(tag.Values[row])
	}
	return size + uvarintSize(uint64(len(batch.Tags)))
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
