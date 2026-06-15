package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

func writeBlock(file *os.File, payload []byte) (blockRef, error) {
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return blockRef{}, fmt.Errorf("seek block writer: %w", err)
	}
	frame := make([]byte, 4+len(payload)+4)
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	checksum := crc32.Checksum(payload, crcTable)
	binary.BigEndian.PutUint32(frame[len(frame)-4:], checksum)
	if _, err := file.Write(frame); err != nil {
		return blockRef{}, fmt.Errorf("write block: %w", err)
	}
	return blockRef{
		Offset: offset,
		Size:   int64(len(frame)),
	}, nil
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
