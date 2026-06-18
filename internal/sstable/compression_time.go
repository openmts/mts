package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

func encodeTimestamps(timestamps []int64, policy string) (byte, []byte, error) {
	plain := appendPlainTimestamps(make([]byte, 0, timestampPayloadCapacity(len(timestamps))), timestamps)
	if compressionPolicy(policy, "delta-of-delta") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDeltaOfDeltaTimestamps(make([]byte, 0, timestampPayloadCapacity(len(timestamps))), timestamps)
	if len(candidate) < len(plain) {
		return compressionDeltaOfDelta, candidate, nil
	}
	return compressionPlain, plain, nil
}

func encodeSampleTimestamps(samples []model.VersionedSample, policy string) (byte, []byte, error) {
	plain := appendPlainSampleTimestamps(make([]byte, 0, timestampPayloadCapacity(len(samples))), samples)
	if compressionPolicy(policy, "delta-of-delta") == "plain" {
		return compressionPlain, plain, nil
	}
	candidate := appendDeltaOfDeltaSampleTimestamps(make([]byte, 0, timestampPayloadCapacity(len(samples))), samples)
	if len(candidate) < len(plain) {
		return compressionDeltaOfDelta, candidate, nil
	}
	return compressionPlain, plain, nil
}

func timestampPayloadCapacity(count int) int {
	if count == 0 {
		return 0
	}
	return 8 + (count-1)*binary.MaxVarintLen64
}

func appendPlainTimestamps(dst []byte, timestamps []int64) []byte {
	if len(timestamps) == 0 {
		return dst
	}
	dst = binary.LittleEndian.AppendUint64(dst, uint64(timestamps[0]))
	for index := 1; index < len(timestamps); index++ {
		dst = binary.AppendVarint(dst, timestamps[index]-timestamps[index-1])
	}
	return dst
}

func appendPlainSampleTimestamps(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	dst = binary.LittleEndian.AppendUint64(dst, uint64(samples[0].Timestamp))
	for index := 1; index < len(samples); index++ {
		dst = binary.AppendVarint(dst, samples[index].Timestamp-samples[index-1].Timestamp)
	}
	return dst
}

func appendDeltaOfDeltaTimestamps(dst []byte, timestamps []int64) []byte {
	if len(timestamps) == 0 {
		return dst
	}
	dst = binary.LittleEndian.AppendUint64(dst, uint64(timestamps[0]))
	if len(timestamps) == 1 {
		return dst
	}
	prevDelta := timestamps[1] - timestamps[0]
	dst = binary.AppendVarint(dst, prevDelta)
	for index := 2; index < len(timestamps); {
		delta := timestamps[index] - timestamps[index-1]
		dd := delta - prevDelta
		if dd != 0 {
			dst = binary.AppendUvarint(dst, zigZag64(dd)<<1|1)
			prevDelta = delta
			index++
			continue
		}
		run := countEqualDeltaRun(timestamps, index, prevDelta)
		dst = append(dst, 0)
		dst = binary.AppendUvarint(dst, uint64(run))
		index += run
	}
	return dst
}

func appendDeltaOfDeltaSampleTimestamps(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	dst = binary.LittleEndian.AppendUint64(dst, uint64(samples[0].Timestamp))
	if len(samples) == 1 {
		return dst
	}
	prevDelta := samples[1].Timestamp - samples[0].Timestamp
	dst = binary.AppendVarint(dst, prevDelta)
	for index := 2; index < len(samples); {
		delta := samples[index].Timestamp - samples[index-1].Timestamp
		dd := delta - prevDelta
		if dd != 0 {
			dst = binary.AppendUvarint(dst, zigZag64(dd)<<1|1)
			prevDelta = delta
			index++
			continue
		}
		run := countEqualSampleDeltaRun(samples, index, prevDelta)
		dst = append(dst, 0)
		dst = binary.AppendUvarint(dst, uint64(run))
		index += run
	}
	return dst
}

func countEqualSampleDeltaRun(samples []model.VersionedSample, index int, prevDelta int64) int {
	run := 1
	for index+run < len(samples) {
		nextDelta := samples[index+run].Timestamp - samples[index+run-1].Timestamp
		if nextDelta != prevDelta {
			break
		}
		run++
	}
	return run
}

func countEqualDeltaRun(timestamps []int64, index int, prevDelta int64) int {
	run := 1
	for index+run < len(timestamps) {
		nextDelta := timestamps[index+run] - timestamps[index+run-1]
		if nextDelta != prevDelta {
			break
		}
		run++
	}
	return run
}

func readCodecTimestamps(reader *blockReader, count int) ([]int64, error) {
	codecID, payload, err := readCodecPayload(reader, "timestamps")
	if err != nil {
		return nil, err
	}
	return decodeCodecTimestamps(codecID, payload, count)
}

func decodeCodecTimestamps(codecID byte, payload []byte, count int) ([]int64, error) {
	switch codecID {
	case compressionPlain:
		return decodePlainTimestamps(payload, count)
	case compressionDeltaOfDelta:
		return decodeDeltaOfDeltaTimestamps(payload, count)
	default:
		return nil, fmt.Errorf("unknown timestamp compression %d", codecID)
	}
}

func decodePlainTimestamps(payload []byte, count int) ([]int64, error) {
	return readTimestamps(newBlockReader(payload), count)
}

func decodeDeltaOfDeltaTimestamps(payload []byte, count int) ([]int64, error) {
	reader := newBlockReader(payload)
	timestamps, err := readDeltaOfDeltaTimestampValues(reader, count)
	if err != nil {
		return nil, err
	}
	return timestamps, reader.done("delta-of-delta timestamps")
}

func readDeltaOfDeltaTimestampValues(reader *blockReader, count int) ([]int64, error) {
	if count == 0 {
		return nil, nil
	}
	timestamps, prevDelta, err := readDeltaOfDeltaPrefix(reader, count)
	if err != nil || count <= 2 {
		return timestamps, err
	}
	return readDeltaOfDeltaTail(reader, timestamps, prevDelta)
}

func readDeltaOfDeltaPrefix(
	reader *blockReader,
	count int,
) ([]int64, int64, error) {
	first, err := reader.fixedInt64("first timestamp")
	if err != nil {
		return nil, 0, err
	}
	timestamps := make([]int64, count)
	timestamps[0] = first
	if count == 1 {
		return timestamps, 0, nil
	}
	prevDelta, err := reader.varint("first timestamp delta")
	if err != nil {
		return nil, 0, err
	}
	timestamps[1] = timestamps[0] + prevDelta
	return timestamps, prevDelta, nil
}

func readDeltaOfDeltaTail(
	reader *blockReader,
	timestamps []int64,
	prevDelta int64,
) ([]int64, error) {
	for index := 2; index < len(timestamps); {
		tag, err := reader.uvarint("timestamp delta-of-delta tag")
		if err != nil {
			return nil, err
		}
		run, dd, err := decodeDeltaOfDeltaTag(reader, tag)
		if err != nil {
			return nil, err
		}
		for range run {
			if index >= len(timestamps) {
				return nil, fmt.Errorf("timestamp delta run exceeds count")
			}
			prevDelta += dd
			timestamps[index] = timestamps[index-1] + prevDelta
			index++
		}
	}
	return timestamps, nil
}

func decodeDeltaOfDeltaTag(reader *blockReader, tag uint64) (int, int64, error) {
	if tag != 0 {
		return 1, unzigZag64(tag >> 1), nil
	}
	run64, err := reader.uvarint("timestamp zero delta run")
	if err != nil {
		return 0, 0, err
	}
	if run64 == 0 || run64 > uint64(int(^uint(0)>>1)) {
		return 0, 0, fmt.Errorf("invalid timestamp zero delta run %d", run64)
	}
	return int(run64), 0, nil
}
