package wal

import (
	"github.com/openmts/mts/internal/model"
)

func estimateTypedBatchSize(
	batch model.ResolvedTypedBatch,
	rows []int,
	identityRows []int,
) int {
	size := uvarintSize(uint64(len(identityRows)))
	for _, row := range identityRows {
		size += estimateTypedIdentitySize(batch, row)
	}
	size += uvarintSize(uint64(len(batch.Fields)))
	for _, field := range batch.Fields {
		size += stringSize(field.Name)
	}
	for position := range typedRowCount(batch, rows) {
		size += estimateTypedPointSize(batch, typedRowIndex(rows, position))
	}
	return size
}

func estimateTypedIdentitySize(batch model.ResolvedTypedBatch, row int) int {
	size := stringSize(batch.Database)
	size += stringSize(batch.RetentionPolicy)
	size += stringSize(batch.Measurement)
	for _, tag := range batch.Tags {
		size += stringSize(tag.Name) + stringSize(tag.Values[row])
	}
	return size + uvarintSize(uint64(len(batch.Tags)))
}

func estimateTypedPointSize(batch model.ResolvedTypedBatch, row int) int {
	size := uvarintSize(0) + uvarintSize(batch.SeriesIDs[row])
	size += varintSize(batch.Timestamps[row])
	size += uvarintSize(batch.WriteSeqs[row])
	size += uvarintSize(uint64(len(batch.Fields)))
	for _, field := range batch.Fields {
		size += estimateTypedFieldSize(field, row)
	}
	return size
}

func estimateTypedFieldSize(field model.ResolvedTypedFieldColumn, row int) int {
	size := uvarintSize(uint64(field.FieldID)) + uvarintSize(0) + 1
	switch field.Type {
	case model.FieldFloat64:
		return size + 8
	case model.FieldInt64:
		return size + varintSize(field.Int64Values[row])
	case model.FieldString:
		return size + stringSize(field.StringValues[row])
	case model.FieldBool:
		return size + 1
	default:
		return size
	}
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
