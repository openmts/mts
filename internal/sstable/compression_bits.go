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
// 使用缓冲 bit 寄存器，避免 Gorilla 热路径中逐位 /8 %8 计算。
type bitReader struct {
	data []byte
	byte int
	bit  int // 当前字节内已消费位数 [0,8)
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) readBit() (uint64, error) {
	if r.byte >= len(r.data) {
		return 0, fmt.Errorf("truncated bit stream")
	}
	shift := 7 - r.bit
	bit := uint64((r.data[r.byte] >> uint(shift)) & 1)
	r.bit++
	if r.bit == 8 {
		r.bit = 0
		r.byte++
	}
	return bit, nil
}

func (r *bitReader) readBits(n int) (uint64, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 64 {
		return 0, fmt.Errorf("readBits n=%d exceeds 64", n)
	}
	var value uint64
	for n > 0 {
		if r.byte >= len(r.data) {
			return 0, fmt.Errorf("truncated bit stream")
		}
		avail := 8 - r.bit
		take := n
		if take > avail {
			take = avail
		}
		// 从当前字节 MSB 侧取 take 位。
		shift := avail - take
		mask := byte((1 << uint(take)) - 1)
		chunk := uint64((r.data[r.byte] >> uint(shift)) & mask)
		value = (value << uint(take)) | chunk
		r.bit += take
		if r.bit == 8 {
			r.bit = 0
			r.byte++
		}
		n -= take
	}
	return value, nil
}

func (r *bitReader) bitsRead() int {
	return r.byte*8 + r.bit
}

func (r *bitReader) consumeAligned(reader *blockReader) error {
	usedBytes := (r.bitsRead() + 7) / 8
	if usedBytes > len(r.data) {
		return fmt.Errorf("truncated bit stream payload")
	}
	if usedBytes > len(reader.rest) {
		return fmt.Errorf("truncated bit stream payload")
	}
	reader.rest = reader.rest[usedBytes:]
	return nil
}
