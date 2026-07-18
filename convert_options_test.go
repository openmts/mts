package mts

import (
	"log/slog"
	"testing"
	"time"
)

func TestToModelOptionsPreservesStorageConfiguration(t *testing.T) {
	logger := testLogger()
	opts := Options{
		Path:                   "data",
		DefaultDatabase:        "metrics",
		DefaultRetentionPolicy: "rp",
		ShardDuration:          2 * time.Hour,
		Retention:              24 * time.Hour,
		MemTableMaxSamples:     123,
		FlushSync:              true,
		WAL: WALOptions{
			Sync:          true,
			SegmentBytes:  1024,
			BatchRecords:  10,
			BatchBytes:    2048,
			BatchInterval: time.Second,
			Logger:        logger,
		},
		Compaction: CompactionOptions{
			Enabled:                    true,
			Level0PartLimit:            3,
			Level0SizeLimit:            4096,
			MaxOutputPartBytes:         8192,
			MaxConcurrent:              2,
			MaxCascadeSteps:            4,
			BackgroundInterval:         time.Minute,
			ReadAmplificationPartLimit: 5,
			BacklogDegradedThreshold:   6,
			DiskSpaceReserveBytes:      7,
			MinFreeBytes:               8,
			Levels: []CompactionLevelOptions{{
				Level:              1,
				PartLimit:          9,
				SizeLimit:          10,
				MaxOutputPartBytes: 11,
				Compression: CompressionOptions{
					Enabled:          true,
					Algorithm:        "zstd",
					Timestamp:        "delta",
					Float:            "xor",
					Int:              "zigzag",
					String:           "dict",
					MinPageValues:    12,
					ValuePageSamples: 256,
				},
			}},
		},
		Compression: CompressionOptions{
			Enabled:          true,
			Algorithm:        "snappy",
			Timestamp:        "delta",
			Float:            "xor",
			Int:              "zigzag",
			String:           "plain",
			MinPageValues:    13,
			ValuePageSamples: 1024,
		},
		StorageMemory: StorageMemoryOptions{
			SoftSampleLimit:       14,
			HardSampleLimit:       15,
			SoftBytesLimit:        16,
			HardBytesLimit:        17,
			QueryBytesLimit:       18,
			FlushBytesLimit:       19,
			CompactionBytesLimit:  20,
			CompressionBytesLimit: 21,
		},
		MaxConcurrentDownsample: 4,
		MaxConcurrentCompaction: 2,
		Logger:                  logger,
		User: UserOptions{
			Endpoint:             "local",
			PasswordAuthDisabled: true,
		},
	}

	got := toModelOptions(opts)

	if got.Path != opts.Path ||
		got.DefaultDatabase != opts.DefaultDatabase ||
		got.DefaultRetentionPolicy != opts.DefaultRetentionPolicy ||
		got.ShardDuration != opts.ShardDuration ||
		got.Retention != opts.Retention ||
		got.MemTableMaxSamples != opts.MemTableMaxSamples ||
		got.FlushSync != opts.FlushSync ||
		got.MaxConcurrentDownsample != opts.MaxConcurrentDownsample ||
		got.MaxConcurrentCompaction != opts.MaxConcurrentCompaction ||
		got.Logger != logger {
		t.Fatalf("toModelOptions() basic fields = %#v", got)
	}
	if got.WAL.Sync != opts.WAL.Sync ||
		got.WAL.SegmentBytes != opts.WAL.SegmentBytes ||
		got.WAL.BatchRecords != opts.WAL.BatchRecords ||
		got.WAL.BatchBytes != opts.WAL.BatchBytes ||
		got.WAL.BatchInterval != opts.WAL.BatchInterval ||
		got.WAL.Logger != logger {
		t.Fatalf("toModelOptions() WAL = %#v", got.WAL)
	}
	if got.Compaction.Enabled != opts.Compaction.Enabled ||
		got.Compaction.Level0PartLimit != opts.Compaction.Level0PartLimit ||
		got.Compaction.Level0SizeLimit != opts.Compaction.Level0SizeLimit ||
		got.Compaction.MaxOutputPartBytes != opts.Compaction.MaxOutputPartBytes ||
		got.Compaction.MaxConcurrent != opts.Compaction.MaxConcurrent ||
		got.Compaction.MaxCascadeSteps != opts.Compaction.MaxCascadeSteps ||
		got.Compaction.BackgroundInterval != opts.Compaction.BackgroundInterval ||
		got.Compaction.ReadAmplificationPartLimit != opts.Compaction.ReadAmplificationPartLimit ||
		got.Compaction.BacklogDegradedThreshold != opts.Compaction.BacklogDegradedThreshold ||
		got.Compaction.DiskSpaceReserveBytes != opts.Compaction.DiskSpaceReserveBytes ||
		got.Compaction.MinFreeBytes != opts.Compaction.MinFreeBytes ||
		len(got.Compaction.Levels) != 1 {
		t.Fatalf("toModelOptions() compaction = %#v", got.Compaction)
	}
	level := got.Compaction.Levels[0]
	if level.Level != 1 || level.PartLimit != 9 || level.SizeLimit != 10 ||
		level.MaxOutputPartBytes != 11 || level.Compression.Algorithm != "zstd" {
		t.Fatalf("toModelOptions() compaction level = %#v", level)
	}
	if got.Compression.Algorithm != "snappy" ||
		got.Compression.MinPageValues != 13 ||
		got.Compression.ValuePageSamples != 1024 ||
		got.StorageMemory.QueryBytesLimit != 18 ||
		got.StorageMemory.CompressionBytesLimit != 21 {
		t.Fatalf("toModelOptions() compression/memory = %#v %#v", got.Compression, got.StorageMemory)
	}
	if got.Compaction.Levels[0].Compression.ValuePageSamples != 256 {
		t.Fatalf("toModelOptions() level page samples = %d", got.Compaction.Levels[0].Compression.ValuePageSamples)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
