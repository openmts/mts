package mts

import "github.com/openmts/mts/internal/model"

func toModelOptions(opts Options) model.Options {
	return model.Options{
		Path:                    opts.Path,
		DefaultDatabase:         opts.DefaultDatabase,
		DefaultRetentionPolicy:  opts.DefaultRetentionPolicy,
		ShardDuration:           opts.ShardDuration,
		Retention:               opts.Retention,
		MemTableMaxSamples:      opts.MemTableMaxSamples,
		WAL:                     toModelWALOptions(opts.WAL),
		FlushSync:               opts.FlushSync,
		Compaction:              toModelCompactionOptions(opts.Compaction),
		Compression:             toModelCompressionOptions(opts.Compression),
		StorageMemory:           toModelStorageMemoryOptions(opts.StorageMemory),
		Cardinality:             toModelCardinalityOptions(opts.Cardinality),
		MaxConcurrentDownsample: opts.MaxConcurrentDownsample,
		MaxConcurrentCompaction: opts.MaxConcurrentCompaction,
		QueryProtection: model.QueryProtectionOptions{
			DefaultMaxSamples: opts.QueryProtection.DefaultMaxSamples,
			DefaultLimit:      opts.QueryProtection.DefaultLimit,
		},
		MemTableDisorderFlushRatio:      opts.MemTableDisorderFlushRatio,
		MemTableDisorderFlushMinSamples: opts.MemTableDisorderFlushMinSamples,
		Logger:                          opts.Logger,
	}
}

func toModelCardinalityOptions(opts CardinalityOptions) model.CardinalityOptions {
	return model.CardinalityOptions{
		MaxSeries:          opts.MaxSeries,
		MaxFields:          opts.MaxFields,
		MaxTagValuesPerKey: opts.MaxTagValuesPerKey,
	}
}

func toModelStorageMemoryOptions(opts StorageMemoryOptions) model.StorageMemoryOptions {
	return model.StorageMemoryOptions{
		SoftSampleLimit:       opts.SoftSampleLimit,
		HardSampleLimit:       opts.HardSampleLimit,
		SoftBytesLimit:        opts.SoftBytesLimit,
		HardBytesLimit:        opts.HardBytesLimit,
		QueryBytesLimit:       opts.QueryBytesLimit,
		FlushBytesLimit:       opts.FlushBytesLimit,
		CompactionBytesLimit:  opts.CompactionBytesLimit,
		CompressionBytesLimit: opts.CompressionBytesLimit,
	}
}

func toModelWALOptions(opts WALOptions) model.WALOptions {
	return model.WALOptions{
		Sync:          opts.Sync,
		SegmentBytes:  opts.SegmentBytes,
		BatchRecords:  opts.BatchRecords,
		BatchBytes:    opts.BatchBytes,
		BatchInterval: opts.BatchInterval,
		Logger:        opts.Logger,
	}
}

func toModelCompactionOptions(opts CompactionOptions) model.CompactionOptions {
	levels := make([]model.CompactionLevelOptions, len(opts.Levels))
	for index, level := range opts.Levels {
		levels[index] = model.CompactionLevelOptions{
			Level:              level.Level,
			PartLimit:          level.PartLimit,
			SizeLimit:          level.SizeLimit,
			MaxOutputPartBytes: level.MaxOutputPartBytes,
			Compression:        toModelCompressionOptions(level.Compression),
		}
	}
	return model.CompactionOptions{
		Enabled:                    opts.Enabled,
		Level0PartLimit:            opts.Level0PartLimit,
		Level0SizeLimit:            opts.Level0SizeLimit,
		MaxOutputPartBytes:         opts.MaxOutputPartBytes,
		MaxConcurrent:              opts.MaxConcurrent,
		Levels:                     levels,
		MaxCascadeSteps:            opts.MaxCascadeSteps,
		BackgroundInterval:         opts.BackgroundInterval,
		ReadAmplificationPartLimit: opts.ReadAmplificationPartLimit,
		BacklogDegradedThreshold:   opts.BacklogDegradedThreshold,
		DiskSpaceReserveBytes:      opts.DiskSpaceReserveBytes,
		MinFreeBytes:               opts.MinFreeBytes,
	}
}

func toModelCompressionOptions(opts CompressionOptions) model.CompressionOptions {
	return model.CompressionOptions{
		Enabled:          opts.Enabled,
		Timestamp:        opts.Timestamp,
		Float:            opts.Float,
		Int:              opts.Int,
		String:           opts.String,
		Algorithm:        opts.Algorithm,
		MinPageValues:    opts.MinPageValues,
		ValuePageSamples: opts.ValuePageSamples,
		OmitWriteSeq:     opts.OmitWriteSeq,
		ZstdLevel:        opts.ZstdLevel,
	}
}
