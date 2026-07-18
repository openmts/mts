package sstable

import "fmt"

// bitWriter 按 MSB-first 写入位流，末尾不足一字节时右侧补 0。
type bitWriter struct {
	buf   []byte
	cur   byte
	nbits int
}

func newBitWriter(dst []byte) *bitWriter {
	return &bitWriter{buf: dst}
}

func (w *bitWriter) writeBit(bit uint64) {
	w.cur = (w.cur << 1) | byte(bit&1)
	w.nbits++
	if w.nbits == 8 {
		w.buf = append(w.buf, w.cur)
		w.cur = 0
		w.nbits = 0
	}
}

func (w *bitWriter) writeBits(value uint64, n int) {
	for index := n - 1; index >= 0; index-- {
		w.writeBit((value >> uint(index)) & 1)
	}
}

func (w *bitWriter) bytes() []byte {
	if w.nbits == 0 {
		return w.buf
	}
	return append(w.buf, w.cur<<uint(8-w.nbits))
}

// bitReader 按 MSB-first 读取位流。
type bitReader struct {
	data []byte
	pos  int
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) readBit() (uint64, error) {
	byteIndex := r.pos / 8
	if byteIndex >= len(r.data) {
		return 0, fmt.Errorf("truncated bit stream")
	}
	shift := 7 - (r.pos % 8)
	bit := uint64((r.data[byteIndex] >> uint(shift)) & 1)
	r.pos++
	return bit, nil
}

func (r *bitReader) readBits(n int) (uint64, error) {
	if n <= 0 {
		return 0, nil
	}
	var value uint64
	for index := 0; index < n; index++ {
		bit, err := r.readBit()
		if err != nil {
			return 0, err
		}
		value = (value << 1) | bit
	}
	return value, nil
}

func (r *bitReader) consumeAligned(reader *blockReader) error {
	usedBytes := (r.pos + 7) / 8
	if usedBytes > len(r.data) {
		return fmt.Errorf("truncated bit stream payload")
	}
	if usedBytes > len(reader.rest) {
		return fmt.Errorf("truncated bit stream payload")
	}
	reader.rest = reader.rest[usedBytes:]
	return nil
}
