package engine

import (
	"fmt"
	"path/filepath"
	"time"

	"codeberg.org/mts/mts/internal/model"
)

const (
	defaultDatabase        = "default"
	defaultRetentionPolicy = "autogen"
	defaultShardDuration   = time.Hour
	defaultMemTableSamples = 10000
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
	if opts.Compaction.Enabled && opts.Compaction.Level0PartLimit <= 0 {
		opts.Compaction.Level0PartLimit = 4
	}
	return opts
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
