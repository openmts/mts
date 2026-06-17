package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"

	"codeberg.org/mts/mts/internal/storagefs"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

const maxPooledBlockFrameBytes = 1 << 20

var blockFramePool = sync.Pool{
	New: func() any {
		return &blockFrame{buf: make([]byte, 0, 4096)}
	},
}

type blockFrame struct {
	buf []byte
}

type blockWriter struct {
	file   *os.File
	offset int64
}

type blockPayload struct {
	data  []byte
	frame *blockFrame
}

func newBlockWriter(file *os.File) (*blockWriter, error) {
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("seek block writer: %w", err)
	}
	return &blockWriter{
		file:   file,
		offset: offset,
	}, nil
}

func (w *blockWriter) write(payload []byte) (blockRef, error) {
	ref, err := writeBlockAt(w.file, w.offset, payload)
	if err != nil {
		return blockRef{}, err
	}
	w.offset += ref.Size
	return ref, nil
}

func writeBlock(file *os.File, payload []byte) (blockRef, error) {
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return blockRef{}, fmt.Errorf("seek block writer: %w", err)
	}
	return writeBlockAt(file, offset, payload)
}

func writeBlockAt(file *os.File, offset int64, payload []byte) (blockRef, error) {
	if uint64(len(payload)) > uint64(^uint32(0)) {
		return blockRef{}, fmt.Errorf("block payload too large: %d", len(payload))
	}
	handle := borrowBlockFrameHandle(len(payload) + 8)
	defer releaseBlockFrameHandle(handle)
	frame := handle.buf
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	checksum := crc32.Checksum(payload, crcTable)
	binary.BigEndian.PutUint32(frame[len(frame)-4:], checksum)
	if err := writeAll(file, frame); err != nil {
		return blockRef{}, err
	}
	return blockRef{
		Offset: offset,
		Size:   int64(len(payload) + 8),
	}, nil
}

func borrowBlockFrame(size int) []byte {
	return borrowBlockFrameHandle(size).buf
}

func borrowBlockFrameHandle(size int) *blockFrame {
	if size > maxPooledBlockFrameBytes {
		return &blockFrame{buf: make([]byte, size)}
	}
	frame, ok := blockFramePool.Get().(*blockFrame)
	if !ok {
		return &blockFrame{buf: make([]byte, size)}
	}
	if cap(frame.buf) < size {
		frame.buf = make([]byte, size)
		return frame
	}
	frame.buf = frame.buf[:size]
	return frame
}

func releaseBlockFrame(frame []byte) {
	if cap(frame) > maxPooledBlockFrameBytes {
		return
	}
	releaseBlockFrameHandle(&blockFrame{buf: frame[:0]})
}

func releaseBlockFrameHandle(frame *blockFrame) {
	if frame == nil || cap(frame.buf) > maxPooledBlockFrameBytes {
		return
	}
	frame.buf = frame.buf[:0]
	blockFramePool.Put(frame)
}

func writeAll(file *os.File, data []byte) error {
	if err := storagefs.WriteFull(file, data); err != nil {
		return fmt.Errorf("write block: %w", err)
	}
	return nil
}

func readBlock(path string, ref blockRef) ([]byte, error) {
	file, err := storagefs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open block file: %w", err)
	}
	payload, readErr := readBlockFrom(file, ref)
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close block file: %w", closeErr)
	}
	return payload, nil
}

func readBlockFrom(file *os.File, ref blockRef) ([]byte, error) {
	payload, err := readBlockPayloadFrom(file, ref)
	if err != nil {
		return nil, err
	}
	data := append([]byte(nil), payload.Bytes()...)
	payload.Release()
	return data, nil
}

func readBlockPayloadFrom(file *os.File, ref blockRef) (blockPayload, error) {
	if ref.Size < 8 {
		return blockPayload{}, fmt.Errorf("block frame is too small")
	}
	frame := borrowBlockFrameHandle(int(ref.Size))
	if _, err := file.ReadAt(frame.buf, ref.Offset); err != nil {
		releaseBlockFrameHandle(frame)
		return blockPayload{}, fmt.Errorf("read block: %w", err)
	}
	length := binary.BigEndian.Uint32(frame.buf[:4])
	if int(length) != len(frame.buf)-8 {
		releaseBlockFrameHandle(frame)
		return blockPayload{}, fmt.Errorf("block length mismatch")
	}
	payload := frame.buf[4 : len(frame.buf)-4]
	want := binary.BigEndian.Uint32(frame.buf[len(frame.buf)-4:])
	got := crc32.Checksum(payload, crcTable)
	if got != want {
		releaseBlockFrameHandle(frame)
		return blockPayload{}, fmt.Errorf("block crc mismatch")
	}
	return blockPayload{data: payload, frame: frame}, nil
}

func (p blockPayload) Bytes() []byte {
	return p.data
}

func (p *blockPayload) Release() {
	if p == nil || p.frame == nil {
		return
	}
	releaseBlockFrameHandle(p.frame)
	p.data = nil
	p.frame = nil
}
