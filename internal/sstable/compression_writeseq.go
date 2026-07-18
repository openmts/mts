package sstable

import (
	"encoding/binary"
	"fmt"

	"github.com/openmts/mts/internal/model"
)

// writeSeq 载荷选择：
// 1) plain：逐点 uvarint(seq)
// 2) delta-RLE：first uvarint + 重复 (run_len uvarint, zigzag_delta uvarint)
// 使用 compressionRLE codec id。

func encodeWriteSeqs(samples []model.VersionedSample) (byte, []byte) {
	plain := appendPlainWriteSeqs(make([]byte, 0, len(samples)*binary.MaxVarintLen64), samples)
	if len(samples) < 2 {
		return compressionPlain, plain
	}
	rle := appendDeltaRLEWriteSeqs(make([]byte, 0, 16+len(samples)), samples)
	if len(rle) < len(plain) {
		return compressionRLE, rle
	}
	return compressionPlain, plain
}

func appendPlainWriteSeqs(dst []byte, samples []model.VersionedSample) []byte {
	for _, sample := range samples {
		dst = binary.AppendUvarint(dst, sample.WriteSeq)
	}
	return dst
}

func appendDeltaRLEWriteSeqs(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	prev := samples[0].WriteSeq
	dst = binary.AppendUvarint(dst, prev)
	if len(samples) == 1 {
		return dst
	}
	index := 1
	for index < len(samples) {
		next := samples[index].WriteSeq
		delta := int64(next) - int64(prev)
		run := 1
		prev = next
		index++
		for index < len(samples) {
			candidate := samples[index].WriteSeq
			if int64(candidate)-int64(prev) != delta {
				break
			}
			prev = candidate
			run++
			index++
		}
		dst = binary.AppendUvarint(dst, uint64(run))
		dst = binary.AppendUvarint(dst, zigZag64(delta))
	}
	return dst
}

func decodeWriteSeqs(codecID byte, payload []byte, count int) ([]uint64, error) {
	switch codecID {
	case compressionPlain:
		payloadReader := blockReader{rest: payload}
		writeSeqs, err := readWriteSeqs(&payloadReader, count)
		if err != nil {
			return nil, err
		}
		return writeSeqs, payloadReader.done("write seqs")
	case compressionRLE:
		return decodeDeltaRLEWriteSeqs(payload, count)
	default:
		return nil, fmt.Errorf("unknown write seq compression %d", codecID)
	}
}

func decodeDeltaRLEWriteSeqs(payload []byte, count int) ([]uint64, error) {
	reader := newBlockReader(payload)
	if count == 0 {
		return nil, reader.done("write seq rle")
	}
	first, err := reader.uvarint("first write seq")
	if err != nil {
		return nil, err
	}
	writeSeqs := make([]uint64, count)
	writeSeqs[0] = first
	filled := 1
	prev := first
	for filled < count {
		run64, err := reader.uvarint("write seq rle run")
		if err != nil {
			return nil, err
		}
		if run64 == 0 {
			return nil, fmt.Errorf("write seq rle run must be > 0")
		}
		delta, err := reader.uvarint("write seq rle delta")
		if err != nil {
			return nil, err
		}
		step := unzigZag64(delta)
		for range run64 {
			if filled >= count {
				return nil, fmt.Errorf("write seq rle overflow: filled %d count %d", filled, count)
			}
			next := uint64(int64(prev) + step)
			writeSeqs[filled] = next
			prev = next
			filled++
		}
	}
	return writeSeqs, reader.done("write seq rle")
}
