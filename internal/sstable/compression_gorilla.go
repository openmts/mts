package sstable

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"

	"github.com/openmts/mts/internal/model"
)

// Gorilla float 载荷：首值 8 字节 LE bits + 后续 XOR 位打包流。
// 仍复用 compressionXOR codec id（POC 不兼容旧 uvarint-xor 载荷）。

func appendGorillaFloatValues(dst []byte, samples []model.VersionedSample) []byte {
	if len(samples) == 0 {
		return dst
	}
	prev := math.Float64bits(samples[0].Value.Float64)
	dst = binary.LittleEndian.AppendUint64(dst, prev)
	if len(samples) == 1 {
		return dst
	}
	writer := newBitWriter(dst)
	prevLeading := -1
	prevTrailing := 0
	for index := 1; index < len(samples); index++ {
		next := math.Float64bits(samples[index].Value.Float64)
		prevLeading, prevTrailing = writeGorillaXOR(writer, next^prev, prevLeading, prevTrailing)
		prev = next
	}
	return writer.bytes()
}

func writeGorillaXOR(writer *bitWriter, xor uint64, prevLeading, prevTrailing int) (int, int) {
	if xor == 0 {
		writer.writeBit(0)
		return prevLeading, prevTrailing
	}
	writer.writeBit(1)
	leading := bits.LeadingZeros64(xor)
	if leading > 31 {
		leading = 31
	}
	trailing := bits.TrailingZeros64(xor)
	mbits := 64 - leading - trailing
	if prevLeading >= 0 && leading >= prevLeading && trailing >= prevTrailing {
		writer.writeBit(0)
		window := 64 - prevLeading - prevTrailing
		writer.writeBits(xor>>uint(prevTrailing), window)
		return prevLeading, prevTrailing
	}
	writer.writeBit(1)
	writer.writeBits(uint64(leading), 5)
	writer.writeBits(uint64(mbits-1), 6)
	writer.writeBits(xor>>uint(trailing), mbits)
	return leading, trailing
}

func readGorillaFloatValues(
	reader *blockReader,
	codecID byte,
	count int,
) ([]model.FieldValue, error) {
	if codecID != compressionXOR {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	values := make([]model.FieldValue, count)
	if count == 0 {
		return values, nil
	}
	first, err := reader.fixedInt64("first float bits")
	if err != nil {
		return nil, err
	}
	prev := uint64(first)
	values[0] = model.Float64Value(math.Float64frombits(prev))
	if count == 1 {
		return values, nil
	}
	bitStream := newBitReader(reader.rest)
	prevLeading, prevTrailing := -1, 0
	for index := 1; index < count; index++ {
		prev, prevLeading, prevTrailing, err = readGorillaNext(bitStream, prev, prevLeading, prevTrailing)
		if err != nil {
			return nil, err
		}
		values[index] = model.Float64Value(math.Float64frombits(prev))
	}
	if err := bitStream.consumeAligned(reader); err != nil {
		return nil, err
	}
	return values, nil
}

func readGorillaFloatSampleValues(
	reader *blockReader,
	codecID byte,
	timestamps []int64,
	writeSeqs []uint64,
	query Query,
) ([]model.VersionedSample, error) {
	if codecID != compressionXOR {
		return nil, fmt.Errorf("unknown float compression %d", codecID)
	}
	samples := make([]model.VersionedSample, 0, compressedQueryCapacity(len(timestamps), query))
	if len(timestamps) == 0 {
		return samples, nil
	}
	first, err := reader.fixedInt64("first float bits")
	if err != nil {
		return nil, err
	}
	prev := uint64(first)
	samples = appendCompressedFloatSample(samples, timestamps[0], writeSeqs[0], prev, query)
	if len(timestamps) == 1 {
		return samples, nil
	}
	bitStream := newBitReader(reader.rest)
	prevLeading, prevTrailing := -1, 0
	for index := 1; index < len(timestamps); index++ {
		prev, prevLeading, prevTrailing, err = readGorillaNext(bitStream, prev, prevLeading, prevTrailing)
		if err != nil {
			return nil, err
		}
		samples = appendCompressedFloatSample(samples, timestamps[index], writeSeqs[index], prev, query)
	}
	if err := bitStream.consumeAligned(reader); err != nil {
		return nil, err
	}
	return samples, nil
}

func readGorillaNext(
	reader *bitReader,
	prev uint64,
	prevLeading, prevTrailing int,
) (uint64, int, int, error) {
	control, err := reader.readBit()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("float gorilla control: %w", err)
	}
	if control == 0 {
		return prev, prevLeading, prevTrailing, nil
	}
	reuse, err := reader.readBit()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("float gorilla reuse: %w", err)
	}
	if reuse == 0 {
		if prevLeading < 0 {
			return 0, 0, 0, fmt.Errorf("float gorilla reuse before first block")
		}
		window := 64 - prevLeading - prevTrailing
		bitsValue, err := reader.readBits(window)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("float gorilla reused bits: %w", err)
		}
		return prev ^ (bitsValue << uint(prevTrailing)), prevLeading, prevTrailing, nil
	}
	leading64, err := reader.readBits(5)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("float gorilla leading: %w", err)
	}
	mbitsMinus1, err := reader.readBits(6)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("float gorilla mbits: %w", err)
	}
	mbits := int(mbitsMinus1) + 1
	leading := int(leading64)
	trailing := 64 - leading - mbits
	if trailing < 0 {
		return 0, 0, 0, fmt.Errorf("float gorilla invalid window leading=%d mbits=%d", leading, mbits)
	}
	bitsValue, err := reader.readBits(mbits)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("float gorilla block bits: %w", err)
	}
	xor := bitsValue << uint(trailing)
	return prev ^ xor, leading, trailing, nil
}
