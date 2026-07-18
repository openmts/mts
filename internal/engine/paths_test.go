package engine

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestNormalizeCompressionOptionsDefaults(t *testing.T) {
	opts := normalizeOptions(model.Options{Path: t.TempDir()})
	if opts.Compression.Timestamp != "delta-of-delta" {
		t.Fatalf("Timestamp policy = %q", opts.Compression.Timestamp)
	}
	if opts.Compression.Float != "xor" {
		t.Fatalf("Float policy = %q", opts.Compression.Float)
	}
	if opts.Compression.Int != "delta" {
		t.Fatalf("Int policy = %q", opts.Compression.Int)
	}
	if opts.Compression.String != "dictionary" {
		t.Fatalf("String policy = %q", opts.Compression.String)
	}
}

func TestNormalizeCompressionOptionsPreservesExplicitPlain(t *testing.T) {
	opts := normalizeOptions(model.Options{
		Path: t.TempDir(),
		Compression: model.CompressionOptions{
			Enabled:   true,
			Timestamp: "plain",
			Float:     "plain",
			Int:       "plain",
			String:    "plain",
		},
	})
	if opts.Compression.Float != "plain" || opts.Compression.Timestamp != "plain" {
		t.Fatalf("explicit plain overwritten: %+v", opts.Compression)
	}
}
