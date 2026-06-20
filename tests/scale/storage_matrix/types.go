package main

import "time"

const (
	defaultSizes        = "100k,1m,10m"
	defaultCompressions = "off,none,snappy,lz4,zstd"
	defaultDurabilities = "buffered,wal-sync,write-sync,strict-flush"
)

type scaleSize struct {
	Name   string `json:"name"`
	Points int    `json:"points"`
}

type matrixConfig struct {
	Sizes         []scaleSize
	Compressions  []string
	Durabilities  []string
	DataRoot      string
	Runner        string
	Out           string
	Markdown      string
	Mode          string
	IngestPath    string
	BatchSize     int
	MemTableLimit int
	QueryLimit    int
	ShardDuration time.Duration
	TimestampStep time.Duration
	CaseTimeout   time.Duration
}

type matrixCase struct {
	Size        string `json:"size"`
	Points      int    `json:"points"`
	Compression string `json:"compression_algorithm"`
	Durability  string `json:"durability"`
	DataDir     string `json:"data_dir"`
}

type storageReport struct {
	WriteDurationNanos      int64 `json:"write_duration_nanos"`
	CompactionDurationNanos int64 `json:"compaction_duration_nanos"`
	ColdQueryLatency        int64 `json:"cold_query_latency_nanos"`
	HotQueryLatency         int64 `json:"hot_query_latency_nanos"`
	RSSPeakBytes            int64 `json:"rss_peak_bytes"`
	DataBytes               int64 `json:"data_bytes"`
	ShardCount              int   `json:"shard_count"`
	SSTableBefore           int   `json:"sstable_count_before_compaction"`
	SSTableAfter            int   `json:"sstable_count_after_compaction"`
	Rows                    int   `json:"rows"`
}

type matrixCaseResult struct {
	Case       matrixCase    `json:"case"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration_nanos"`
	Error      string        `json:"error,omitempty"`
	Report     storageReport `json:"report"`
}

type matrixReport struct {
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	DataRoot   string             `json:"data_root"`
	Cases      []matrixCaseResult `json:"cases"`
}
