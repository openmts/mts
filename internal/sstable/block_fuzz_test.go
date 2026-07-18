package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func FuzzBlockRoundTrip(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{1, 2, 3, 4})
	f.Add(bytes.Repeat([]byte{0xab}, 64))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 1<<16 {
			t.Skip("payload too large for fuzz")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "block.bin")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatalf("OpenFile() error = %v", err)
		}
		ref, err := writeBlock(file, payload)
		if err != nil {
			_ = file.Close()
			t.Fatalf("writeBlock() error = %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		got, err := readBlock(path, ref)
		if err != nil {
			t.Fatalf("readBlock() error = %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("round-trip mismatch: got %d bytes want %d", len(got), len(payload))
		}
	})
}

func FuzzBlockCorruptCRCRejected(f *testing.F) {
	f.Add([]byte{1, 2, 3}, byte(0))
	f.Add([]byte{9, 8, 7, 6}, byte(1))
	f.Fuzz(func(t *testing.T, payload []byte, flip byte) {
		if len(payload) > 1<<12 {
			t.Skip("payload too large")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.bin")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatalf("OpenFile() error = %v", err)
		}
		ref, err := writeBlock(file, payload)
		if err != nil {
			_ = file.Close()
			t.Fatalf("writeBlock() error = %v", err)
		}
		// 破坏 CRC 末字节。
		frame := make([]byte, ref.Size)
		if _, err := file.ReadAt(frame, ref.Offset); err != nil {
			_ = file.Close()
			t.Fatalf("ReadAt() error = %v", err)
		}
		frame[len(frame)-1] ^= flip | 1
		if _, err := file.WriteAt(frame, ref.Offset); err != nil {
			_ = file.Close()
			t.Fatalf("WriteAt() error = %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if _, err := readBlock(path, ref); err == nil {
			t.Fatal("readBlock() error = nil, want CRC failure")
		}
	})
}

func TestWriteBlockFrameLayout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	payload := []byte("mts-block")
	ref, err := writeBlock(file, payload)
	if err != nil {
		_ = file.Close()
		t.Fatalf("writeBlock() error = %v", err)
	}
	frame := make([]byte, ref.Size)
	if _, err := file.ReadAt(frame, ref.Offset); err != nil {
		_ = file.Close()
		t.Fatalf("ReadAt() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	length := binary.BigEndian.Uint32(frame[:4])
	if int(length) != len(payload) {
		t.Fatalf("length = %d, want %d", length, len(payload))
	}
	wantCRC := crc32.Checksum(payload, crcTable)
	gotCRC := binary.BigEndian.Uint32(frame[len(frame)-4:])
	if gotCRC != wantCRC {
		t.Fatalf("crc = %d, want %d", gotCRC, wantCRC)
	}
}
