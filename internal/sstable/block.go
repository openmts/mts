package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

const maxPooledBlockFrameBytes = 1 << 20

var blockFramePool = sync.Pool{
	New: func() any {
		buffer := make([]byte, 0, 4096)
		return &buffer
	},
}

func writeBlock(file *os.File, payload []byte) (blockRef, error) {
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return blockRef{}, fmt.Errorf("seek block writer: %w", err)
	}
	if uint64(len(payload)) > uint64(^uint32(0)) {
		return blockRef{}, fmt.Errorf("block payload too large: %d", len(payload))
	}
	frame := borrowBlockFrame(len(payload) + 8)
	defer releaseBlockFrame(frame)
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
	if size > maxPooledBlockFrameBytes {
		return make([]byte, size)
	}
	ptr, ok := blockFramePool.Get().(*[]byte)
	if !ok || ptr == nil {
		return make([]byte, size)
	}
	frame := *ptr
	if cap(frame) < size {
		return make([]byte, size)
	}
	return frame[:size]
}

func releaseBlockFrame(frame []byte) {
	if cap(frame) > maxPooledBlockFrameBytes {
		return
	}
	frame = frame[:0]
	blockFramePool.Put(&frame)
}

func writeAll(file *os.File, data []byte) error {
	for len(data) > 0 {
		written, err := file.Write(data)
		if err != nil {
			return fmt.Errorf("write block: %w", err)
		}
		if written == 0 {
			return fmt.Errorf("write block: wrote zero bytes")
		}
		data = data[written:]
	}
	return nil
}

func readBlock(path string, ref blockRef) ([]byte, error) {
	file, err := os.Open(path)
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
	frame := make([]byte, ref.Size)
	if _, err := file.ReadAt(frame, ref.Offset); err != nil {
		return nil, fmt.Errorf("read block: %w", err)
	}
	if len(frame) < 8 {
		return nil, fmt.Errorf("block frame is too small")
	}
	length := binary.BigEndian.Uint32(frame[:4])
	if int(length) != len(frame)-8 {
		return nil, fmt.Errorf("block length mismatch")
	}
	payload := frame[4 : len(frame)-4]
	want := binary.BigEndian.Uint32(frame[len(frame)-4:])
	got := crc32.Checksum(payload, crcTable)
	if got != want {
		return nil, fmt.Errorf("block crc mismatch")
	}
	return payload, nil
}
