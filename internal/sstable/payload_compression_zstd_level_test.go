package sstable

import (
	"bytes"
	"testing"
)

func TestZstdLevelsRoundTripAndSizeOrdering(t *testing.T) {
	source := bytes.Repeat([]byte("mts-zstd-level-payload-0123456789"), 256)
	levels := []string{"", "fastest", "default", "better", "best"}
	sizes := make(map[string]int, len(levels))
	for _, level := range levels {
		framed, err := appendCodecPayloadWithCompression(nil, compressionPlain, source, "zstd", level)
		if err != nil {
			t.Fatalf("appendCodecPayloadWithCompression(%q) error = %v", level, err)
		}
		codecID, payload, err := readCodecPayload(newBlockReader(framed), "values")
		if err != nil {
			t.Fatalf("readCodecPayload(%q) error = %v", level, err)
		}
		if codecID != compressionPlain {
			t.Fatalf("codecID(%q)=%d, want plain", level, codecID)
		}
		if !bytes.Equal(payload, source) {
			t.Fatalf("round-trip mismatch for level %q", level)
		}
		sizes[level] = len(framed)
	}
	if sizes["best"] > sizes["fastest"] {
		t.Fatalf("best size %d should be <= fastest %d", sizes["best"], sizes["fastest"])
	}
	if sizes["default"] == 0 || sizes[""] == 0 {
		t.Fatal("default/empty sizes missing")
	}
}

func TestResolveZstdEncoderLevel(t *testing.T) {
	if resolveZstdEncoderLevel("") != resolveZstdEncoderLevel("default") {
		t.Fatal("empty level should map to default")
	}
	if resolveZstdEncoderLevel("fastest") == resolveZstdEncoderLevel("best") {
		t.Fatal("fastest and best should differ")
	}
}
