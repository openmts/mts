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

func TestDefaultLevelCompressionUsesTieredAlgorithms(t *testing.T) {
	global := model.CompressionOptions{Enabled: true, MinPageValues: 1, ValuePageSamples: 2048}
	l0 := defaultLevelCompression(0, global)
	if l0.Algorithm != "snappy" {
		t.Fatalf("L0 algorithm = %q, want snappy", l0.Algorithm)
	}
	if l0.ValuePageSamples != 2048 {
		t.Fatalf("L0 page samples = %d, want 2048", l0.ValuePageSamples)
	}
	l1 := defaultLevelCompression(1, global)
	if l1.Algorithm != "zstd" {
		t.Fatalf("L1 algorithm = %q, want zstd", l1.Algorithm)
	}
	if l1.ValuePageSamples != 2048 {
		t.Fatalf("L1 page samples = %d, want inherit global 2048", l1.ValuePageSamples)
	}
	unset := defaultLevelCompression(1, model.CompressionOptions{Enabled: true, MinPageValues: 1})
	if unset.ValuePageSamples != defaultColdTierValuePageSamples {
		t.Fatalf("L1 default page samples = %d, want %d", unset.ValuePageSamples, defaultColdTierValuePageSamples)
	}
	explicit := model.CompressionOptions{Enabled: true, Algorithm: "lz4"}
	if got := defaultLevelCompression(1, explicit); got.Algorithm != "lz4" {
		t.Fatalf("explicit algorithm overwritten: %q", got.Algorithm)
	}
	disabled := defaultLevelCompression(1, model.CompressionOptions{Enabled: false})
	if disabled.Enabled || disabled.Algorithm == "zstd" {
		t.Fatalf("disabled compression mutated: %+v", disabled)
	}
}

func TestNormalizeCompactionLevelInheritsTieredCompression(t *testing.T) {
	global := model.CompressionOptions{Enabled: true, ValuePageSamples: 4096}
	level := normalizeCompactionLevel(model.CompactionLevelOptions{Level: 1}, model.CompactionOptions{}, global)
	if !level.Compression.Enabled || level.Compression.Algorithm != "zstd" {
		t.Fatalf("level compression = %+v, want enabled zstd", level.Compression)
	}
	if level.Compression.ValuePageSamples != defaultColdTierValuePageSamples {
		t.Fatalf("page samples = %d, want cold-tier %d", level.Compression.ValuePageSamples, defaultColdTierValuePageSamples)
	}
}

func TestColdTierValuePageSamples(t *testing.T) {
	if got := coldTierValuePageSamples(0); got != defaultColdTierValuePageSamples {
		t.Fatalf("default = %d", got)
	}
	if got := coldTierValuePageSamples(1024); got != 1024 {
		t.Fatalf("respect configured = %d, want 1024", got)
	}
	if got := coldTierValuePageSamples(32768); got != 32768 {
		t.Fatalf("keep large = %d", got)
	}
}
