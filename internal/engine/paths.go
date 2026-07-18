package engine

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/openmts/mts/internal/model"
)

const (
	defaultDatabase                = "default"
	defaultRetentionPolicy         = "autogen"
	defaultShardDuration           = time.Hour
	defaultMemTableSamples         = 10000
	defaultLevelPartLimit          = 4
	defaultCascadeSteps            = 8
	defaultQueryEndTime            = int64(1<<63 - 1)
	defaultMaxConcurrentDownsample = 2
	defaultMaxConcurrentCompaction = 1
	defaultDisorderFlushMinSamples = 1024
)

func normalizeOptions(opts model.Options) model.Options {
	if opts.DefaultDatabase == "" {
		opts.DefaultDatabase = defaultDatabase
	}
	if opts.DefaultRetentionPolicy == "" {
		opts.DefaultRetentionPolicy = defaultRetentionPolicy
	}
	if opts.ShardDuration <= 0 {
		opts.ShardDuration = defaultShardDuration
	}
	if opts.MemTableMaxSamples <= 0 {
		opts.MemTableMaxSamples = defaultMemTableSamples
	}
	opts.StorageMemory = normalizeStorageMemoryOptions(opts.StorageMemory)
	opts.Compaction = normalizeCompactionOptions(opts.Compaction, opts.Compression)
	if opts.MaxConcurrentDownsample <= 0 {
		opts.MaxConcurrentDownsample = defaultMaxConcurrentDownsample
	}
	if opts.MaxConcurrentCompaction <= 0 {
		opts.MaxConcurrentCompaction = defaultParallelCompactionLimit()
	}
	if opts.MemTableDisorderFlushRatio > 0 && opts.MemTableDisorderFlushMinSamples <= 0 {
		opts.MemTableDisorderFlushMinSamples = defaultDisorderFlushMinSamples
	}
	if opts.Logger == nil {
		opts.Logger = nopLogger()
	}
	if opts.WAL.Logger == nil {
		opts.WAL.Logger = opts.Logger
	}
	return opts
}

func normalizeStorageMemoryOptions(opts model.StorageMemoryOptions) model.StorageMemoryOptions {
	if opts.SoftSampleLimit < 0 {
		opts.SoftSampleLimit = 0
	}
	if opts.HardSampleLimit < 0 {
		opts.HardSampleLimit = 0
	}
	if opts.HardSampleLimit > 0 && opts.SoftSampleLimit > opts.HardSampleLimit {
		opts.SoftSampleLimit = opts.HardSampleLimit
	}
	if opts.SoftBytesLimit < 0 {
		opts.SoftBytesLimit = 0
	}
	if opts.HardBytesLimit < 0 {
		opts.HardBytesLimit = 0
	}
	if opts.QueryBytesLimit < 0 {
		opts.QueryBytesLimit = 0
	}
	if opts.FlushBytesLimit < 0 {
		opts.FlushBytesLimit = 0
	}
	if opts.CompactionBytesLimit < 0 {
		opts.CompactionBytesLimit = 0
	}
	if opts.CompressionBytesLimit < 0 {
		opts.CompressionBytesLimit = 0
	}
	if opts.HardBytesLimit > 0 && opts.SoftBytesLimit > opts.HardBytesLimit {
		opts.SoftBytesLimit = opts.HardBytesLimit
	}
	return opts
}

func normalizeCompactionOptions(
	opts model.CompactionOptions,
	globalCompression model.CompressionOptions,
) model.CompactionOptions {
	if opts.Level0PartLimit <= 0 {
		opts.Level0PartLimit = defaultLevelPartLimit
	}
	if opts.MaxCascadeSteps <= 0 {
		opts.MaxCascadeSteps = defaultCascadeSteps
	}
	levels := append([]model.CompactionLevelOptions(nil), opts.Levels...)
	if len(levels) == 0 {
		levels = append(levels, legacyLevel0Options(opts))
	}
	if !hasCompactionLevel(levels, 0) {
		levels = append(levels, legacyLevel0Options(opts))
	}
	for index := range levels {
		levels[index] = normalizeCompactionLevel(levels[index], opts, globalCompression)
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Level < levels[j].Level
	})
	opts.Levels = levels
	return opts
}

func legacyLevel0Options(opts model.CompactionOptions) model.CompactionLevelOptions {
	return model.CompactionLevelOptions{
		Level:              0,
		PartLimit:          opts.Level0PartLimit,
		SizeLimit:          opts.Level0SizeLimit,
		MaxOutputPartBytes: opts.MaxOutputPartBytes,
	}
}

func hasCompactionLevel(levels []model.CompactionLevelOptions, level int) bool {
	for _, candidate := range levels {
		if candidate.Level == level {
			return true
		}
	}
	return false
}

func normalizeCompactionLevel(
	level model.CompactionLevelOptions,
	opts model.CompactionOptions,
	globalCompression model.CompressionOptions,
) model.CompactionLevelOptions {
	if level.PartLimit <= 0 {
		level.PartLimit = defaultLevelPartLimit
	}
	if level.MaxOutputPartBytes <= 0 {
		level.MaxOutputPartBytes = opts.MaxOutputPartBytes
	}
	if !compressionConfigured(level.Compression) {
		level.Compression = globalCompression
	}
	return level
}

func compressionConfigured(opts model.CompressionOptions) bool {
	return opts.Enabled || opts.Timestamp != "" || opts.Float != "" ||
		opts.Int != "" || opts.String != "" || opts.Algorithm != "" || opts.MinPageValues > 0
}

func normalizePoint(opts model.Options, point model.Point) model.Point {
	if point.Database == "" {
		point.Database = opts.DefaultDatabase
	}
	if point.RetentionPolicy == "" {
		point.RetentionPolicy = opts.DefaultRetentionPolicy
	}
	if point.Tags == nil {
		point.Tags = map[string]string{}
	}
	return point
}

func normalizeTypedBatch(opts model.Options, batch model.TypedBatch) model.TypedBatch {
	if batch.Database == "" {
		batch.Database = opts.DefaultDatabase
	}
	if batch.RetentionPolicy == "" {
		batch.RetentionPolicy = opts.DefaultRetentionPolicy
	}
	return batch
}

func normalizeQuery(opts model.Options, query model.Query) model.Query {
	if query.Database == "" {
		query.Database = opts.DefaultDatabase
	}
	if query.RetentionPolicy == "" {
		query.RetentionPolicy = opts.DefaultRetentionPolicy
	}
	if query.Tags == nil {
		query.Tags = map[string]string{}
	}
	if query.EndTime == 0 {
		query.EndTime = defaultQueryEndTime
	}
	return applyQueryProtection(opts.QueryProtection, query)
}

func applyQueryProtection(protection model.QueryProtectionOptions, query model.Query) model.Query {
	if query.Budget.MaxSamples <= 0 && protection.DefaultMaxSamples > 0 {
		query.Budget.MaxSamples = protection.DefaultMaxSamples
	}
	if query.Limit <= 0 && protection.DefaultLimit > 0 {
		query.Limit = protection.DefaultLimit
	}
	return query
}

func shardStart(timestamp int64, duration time.Duration) int64 {
	size := int64(duration)
	if timestamp >= 0 {
		return timestamp / size * size
	}
	return ((timestamp+1)/size - 1) * size
}

func shardID(database string, policy string, start int64) string {
	return database + "/" + policy + "/" + fmt.Sprint(start)
}

func shardDir(root string, database string, policy string, start int64) string {
	return filepath.Join(root, "data", database, policy, "shards", fmt.Sprint(start))
}

func catalogDir(root string) string {
	return filepath.Join(root, "catalog")
}

func defaultParallelCompactionLimit() int {
	limit := runtime.GOMAXPROCS(0)
	if limit < 1 {
		return defaultMaxConcurrentCompaction
	}
	if limit > 4 {
		return 4
	}
	return limit
}
