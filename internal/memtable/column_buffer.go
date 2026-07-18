package memtable

import (
	"github.com/openmts/mts/internal/model"
)

func columnDataFromBuffer(column *columnBuffer, query Query) model.ColumnData {
	return model.ColumnData{
		SeriesID:  column.seriesID,
		FieldID:   column.fieldID,
		FieldType: column.fieldType,
		Samples:   compactSamples(column, query),
	}
}

func (c *columnBuffer) appendSample(sample model.VersionedSample) int64 {
	if c.fieldType == 0 {
		c.fieldType = sample.Value.Type
	}
	delta := c.reserve(1)
	appendDelta := int64(0)
	oldTimesCap := cap(c.times)
	c.times = append(c.times, sample.Timestamp)
	appendDelta += int64(cap(c.times)-oldTimesCap) * int64Bytes
	oldSeqsCap := cap(c.writeSeqs)
	c.writeSeqs = append(c.writeSeqs, sample.WriteSeq)
	appendDelta += int64(cap(c.writeSeqs)-oldSeqsCap) * uint64Bytes
	appendDelta += c.appendValue(sample.Value)
	c.count++
	c.memBytes += appendDelta
	return delta + appendDelta
}

func (c *columnBuffer) appendValue(value model.FieldValue) int64 {
	switch c.fieldType {
	case model.FieldFloat64:
		oldCap := cap(c.floats)
		c.floats = append(c.floats, value.Float64)
		return int64(cap(c.floats)-oldCap) * float64Bytes
	case model.FieldInt64:
		oldCap := cap(c.ints)
		c.ints = append(c.ints, value.Int64)
		return int64(cap(c.ints)-oldCap) * int64Bytes
	case model.FieldString:
		oldCap := cap(c.strings)
		c.strings = append(c.strings, value.String)
		return int64(cap(c.strings)-oldCap)*stringHeaderBytes + int64(len(value.String))
	case model.FieldBool:
		return c.appendBool(value.Bool)
	}
	return 0
}

func (c *columnBuffer) appendBool(value bool) int64 {
	word := c.count / 64
	oldCap := cap(c.boolBits)
	if word >= len(c.boolBits) {
		c.boolBits = append(c.boolBits, 0)
	}
	if value {
		c.boolBits[word] |= 1 << uint(c.count%64)
	}
	return int64(cap(c.boolBits)-oldCap) * uint64Bytes
}

func (c *columnBuffer) sampleAt(index int) model.VersionedSample {
	return model.VersionedSample{
		Timestamp: c.times[index],
		WriteSeq:  c.writeSeqs[index],
		Value:     c.valueAt(index),
	}
}

func (c *columnBuffer) valueAt(index int) model.FieldValue {
	switch c.fieldType {
	case model.FieldFloat64:
		return model.Float64Value(c.floats[index])
	case model.FieldInt64:
		return model.Int64Value(c.ints[index])
	case model.FieldString:
		return model.StringValue(c.strings[index])
	case model.FieldBool:
		word := index / 64
		bit := uint(index % 64)
		return model.BoolValue(word < len(c.boolBits) && c.boolBits[word]&(1<<bit) != 0)
	default:
		return model.FieldValue{Type: c.fieldType}
	}
}

func (c *columnBuffer) reserve(additional int) int64 {
	if additional <= 0 {
		return 0
	}
	before := c.capacityBytes()
	target := c.count + additional
	c.times = growInt64s(c.times, target, additional)
	c.writeSeqs = growUint64s(c.writeSeqs, target, additional)
	c.reserveValues(target, additional)
	delta := c.capacityBytes() - before
	c.memBytes += delta
	return delta
}

func (c *columnBuffer) reserveValues(target int, additional int) {
	switch c.fieldType {
	case model.FieldFloat64:
		c.floats = growFloat64s(c.floats, target, additional)
	case model.FieldInt64:
		c.ints = growInt64s(c.ints, target, additional)
	case model.FieldString:
		c.strings = growStrings(c.strings, target, additional)
	case model.FieldBool:
		c.boolBits = growUint64s(c.boolBits, boolWords(target), boolWords(additional))
	}
}

func (c *columnBuffer) clear() {
	c.times = nil
	c.writeSeqs = nil
	c.floats = nil
	c.ints = nil
	c.strings = nil
	c.boolBits = nil
	c.count = 0
	c.memBytes = 0
	c.lastTimestamp = 0
	c.hasLast = false
}

func approxTableDataBytes(data tableData) int64 {
	if approxDataHook != nil {
		approxDataHook()
	}
	if data == nil {
		return 0
	}
	total := tableDataBaseBytes + int64(len(data))*mapEntryApproxBytes
	for _, column := range data {
		if column != nil {
			total += column.approxMemoryBytes()
		}
	}
	return total
}

func trackedTableDataBytes(data tableData) int64 {
	if data == nil {
		return 0
	}
	total := tableDataBaseBytes + int64(len(data))*mapEntryApproxBytes
	for _, column := range data {
		if column != nil {
			total += column.memBytes
		}
	}
	return total
}

func (c *columnBuffer) approxMemoryBytes() int64 {
	if c == nil {
		return 0
	}
	total := columnBufferBaseBytes + c.capacityBytes()
	for _, value := range c.strings[:min(c.count, len(c.strings))] {
		total += int64(len(value))
	}
	return total
}

func (c *columnBuffer) capacityBytes() int64 {
	if c == nil {
		return 0
	}
	total := int64(cap(c.times)) * int64Bytes
	total += int64(cap(c.writeSeqs)) * uint64Bytes
	total += int64(cap(c.floats)) * float64Bytes
	total += int64(cap(c.ints)) * int64Bytes
	total += int64(cap(c.strings)) * stringHeaderBytes
	total += int64(cap(c.boolBits)) * uint64Bytes
	return total
}

func growInt64s(values []int64, target int, additional int) []int64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]int64, len(values), next)
	copy(out, values)
	return out
}

func growUint64s(values []uint64, target int, additional int) []uint64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]uint64, len(values), next)
	copy(out, values)
	return out
}

func growFloat64s(values []float64, target int, additional int) []float64 {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]float64, len(values), next)
	copy(out, values)
	return out
}

func growStrings(values []string, target int, additional int) []string {
	if target <= cap(values) {
		return values
	}
	next := nextSliceCapacity(cap(values), target, additional)
	out := make([]string, len(values), next)
	copy(out, values)
	return out
}

func nextSliceCapacity(current int, target int, additional int) int {
	if current == 0 || additional > current {
		return target
	}
	slack := current / 4
	if slack < additional {
		slack = additional
	}
	if slack > 8 {
		slack = 8
	}
	next := target + slack
	if next < target {
		return target
	}
	return next
}

func boolWords(count int) int {
	if count <= 0 {
		return 0
	}
	return (count + 63) / 64
}

func typedBatchRowCount(batch model.ResolvedTypedBatch, rows []int) int {
	if len(rows) > 0 {
		return len(rows)
	}
	return len(batch.Timestamps)
}

func typedBatchRowIndex(rows []int, position int) int {
	if len(rows) > 0 {
		return rows[position]
	}
	return position
}

func typedFieldValueAt(field model.ResolvedTypedFieldColumn, row int) model.FieldValue {
	switch field.Type {
	case model.FieldFloat64:
		return model.Float64Value(field.Float64Values[row])
	case model.FieldInt64:
		return model.Int64Value(field.Int64Values[row])
	case model.FieldString:
		return model.StringValue(field.StringValues[row])
	case model.FieldBool:
		return model.BoolValue(field.BoolValues[row])
	default:
		return model.FieldValue{Type: field.Type}
	}
}

func (c *columnBuffer) appendColumn(src *columnBuffer) int64 {
	if src.count == 0 {
		return 0
	}
	if c.fieldType == 0 {
		c.fieldType = src.fieldType
	}
	delta := c.reserve(src.count)
	for index := range src.count {
		delta += c.appendSample(src.sampleAt(index))
	}
	if src.hasLast {
		c.lastTimestamp = src.lastTimestamp
		c.hasLast = true
	} else if src.count > 0 {
		c.lastTimestamp = src.times[src.count-1]
		c.hasLast = true
	}
	return delta
}
