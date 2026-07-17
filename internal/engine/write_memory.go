package engine

import (
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func resolvedSampleCount(points []model.ResolvedPoint) int {
	total := 0
	for _, point := range points {
		total += len(point.Fields)
	}
	return total
}

func resolvedTypedSampleCount(batch model.ResolvedTypedBatch) int {
	return len(batch.Timestamps) * len(batch.Fields)
}

func estimateResolvedPointsBytes(points []model.ResolvedPoint) int64 {
	var total int64
	for _, point := range points {
		total += int64(32 + len(point.Measurement))
		for key, value := range point.Tags {
			total += int64(len(key) + len(value) + 32)
		}
		for _, field := range point.Fields {
			total += estimateResolvedFieldBytes(field)
		}
	}
	return total
}

func estimateResolvedTypedBatchBytes(batch model.ResolvedTypedBatch, rows []int) int64 {
	var total int64
	for position := range typedBatchRowCount(batch, rows) {
		row := typedBatchRowIndex(rows, position)
		total += int64(32 + len(batch.Measurement))
		for _, tag := range batch.Tags {
			total += int64(len(tag.Name) + len(tag.Values[row]) + 32)
		}
		for _, field := range batch.Fields {
			total += estimateResolvedTypedFieldBytes(field, row)
		}
	}
	return total
}

func estimateWALFrameBytes(points []model.ResolvedPoint) int64 {
	const frameAndRecordOverhead = int64(64)
	return estimateResolvedPointsBytes(points) + frameAndRecordOverhead + int64(len(points))*16
}

func estimateTypedWALFrameBytes(batch model.ResolvedTypedBatch, rows []int) int64 {
	const frameAndRecordOverhead = int64(64)
	count := typedBatchRowCount(batch, rows)
	return estimateResolvedTypedBatchBytes(batch, rows) + frameAndRecordOverhead + int64(count)*16
}

func estimateResolvedFieldBytes(field model.ResolvedField) int64 {
	const sampleBaseBytes = int64(32)
	switch field.Type {
	case model.FieldFloat64, model.FieldInt64:
		return sampleBaseBytes + 8
	case model.FieldString:
		return sampleBaseBytes + 16 + int64(len(field.Value.String))
	case model.FieldBool:
		return sampleBaseBytes + 1
	default:
		return sampleBaseBytes
	}
}

func estimateResolvedTypedFieldBytes(field model.ResolvedTypedFieldColumn, row int) int64 {
	const sampleBaseBytes = int64(32)
	switch field.Type {
	case model.FieldFloat64, model.FieldInt64:
		return sampleBaseBytes + 8
	case model.FieldString:
		return sampleBaseBytes + 16 + int64(len(field.StringValues[row]))
	case model.FieldBool:
		return sampleBaseBytes + 1
	default:
		return sampleBaseBytes
	}
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

func (e *Engine) enforceMemoryBeforeWriteLocked(incomingSamples int, incomingBytes int64) error {
	memory := e.opts.StorageMemory
	if memory.HardSampleLimit > 0 && incomingSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: incoming=%d limit=%d",
			incomingSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && incomingBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, incomingBytes, memory.HardBytesLimit)
	}
	currentSamples := e.totalMemSamplesLocked()
	currentBytes := e.totalMemBytesLocked()
	if shouldFlushBeforeWrite(memory, currentSamples, incomingSamples, currentBytes, incomingBytes) {
		if err := e.flushAllShardsLocked(); err != nil {
			return err
		}
		e.memory.RecordFlushTriggered()
		currentSamples = e.totalMemSamplesLocked()
		currentBytes = e.totalMemBytesLocked()
	}
	if memory.HardSampleLimit > 0 && currentSamples+incomingSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: current=%d incoming=%d limit=%d",
			currentSamples,
			incomingSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && currentBytes+incomingBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, currentBytes+incomingBytes, memory.HardBytesLimit)
	}
	return nil
}

func (e *Engine) enforceMemoryAfterWriteLocked() error {
	memory := e.opts.StorageMemory
	currentSamples := e.totalMemSamplesLocked()
	currentBytes := e.totalMemBytesLocked()
	if shouldFlushAfterWrite(memory, currentSamples, currentBytes) {
		if err := e.flushAllShardsLocked(); err != nil {
			return err
		}
		e.memory.RecordFlushTriggered()
		currentSamples = e.totalMemSamplesLocked()
		currentBytes = e.totalMemBytesLocked()
	}
	if memory.HardSampleLimit > 0 && currentSamples > memory.HardSampleLimit {
		return fmt.Errorf(
			"storage memory hard sample limit exceeded: current=%d limit=%d",
			currentSamples,
			memory.HardSampleLimit,
		)
	}
	if memory.HardBytesLimit > 0 && currentBytes > memory.HardBytesLimit {
		return storageMemoryLimitError(storageMemoryWrite, currentBytes, memory.HardBytesLimit)
	}
	return nil
}

func shouldFlushBeforeWrite(
	memory model.StorageMemoryOptions,
	currentSamples int,
	incomingSamples int,
	currentBytes int64,
	incomingBytes int64,
) bool {
	if incomingSamples == 0 && incomingBytes == 0 {
		return false
	}
	if memory.HardSampleLimit > 0 && currentSamples+incomingSamples > memory.HardSampleLimit {
		return true
	}
	if memory.HardBytesLimit > 0 && currentBytes+incomingBytes > memory.HardBytesLimit {
		return true
	}
	if memory.SoftSampleLimit > 0 && currentSamples+incomingSamples >= memory.SoftSampleLimit && currentSamples > 0 {
		return true
	}
	return memory.SoftBytesLimit > 0 && currentBytes+incomingBytes >= memory.SoftBytesLimit && currentBytes > 0
}

func shouldFlushAfterWrite(memory model.StorageMemoryOptions, currentSamples int, currentBytes int64) bool {
	if memory.SoftSampleLimit > 0 && currentSamples >= memory.SoftSampleLimit {
		return true
	}
	return memory.SoftBytesLimit > 0 && currentBytes >= memory.SoftBytesLimit
}

func (e *Engine) flushAllShardsLocked() error {
	for _, shard := range e.shards {
		if err := shard.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) totalMemSamplesLocked() int {
	total := 0
	for _, shard := range e.shards {
		total += shard.ApproxMemorySamples()
	}
	return total
}

func (e *Engine) totalMemBytesLocked() int64 {
	return e.storageMemoryActiveLocked().total()
}

func (e *Engine) storageMemoryActiveLocked() storageMemoryActive {
	var active storageMemoryActive
	for _, shard := range e.shards {
		memBytes := shard.ApproxMemTableMemoryBytes()
		walBytes := shard.ApproxWALMemoryBytes()
		active.MemTableBytes += memBytes
		active.WALBytes += walBytes
	}
	return active
}
