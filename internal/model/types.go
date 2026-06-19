package model

import "time"

type FieldType uint8

const (
	FieldFloat64 FieldType = iota + 1
	FieldInt64
	FieldString
	FieldBool
)

type FieldValue struct {
	Type    FieldType `json:"type"`
	Float64 float64   `json:"float64,omitempty"`
	Int64   int64     `json:"int64,omitempty"`
	String  string    `json:"string,omitempty"`
	Bool    bool      `json:"bool,omitempty"`
}

func Float64Value(value float64) FieldValue {
	return FieldValue{
		Type:    FieldFloat64,
		Float64: value,
	}
}

func Int64Value(value int64) FieldValue {
	return FieldValue{
		Type:  FieldInt64,
		Int64: value,
	}
}

func StringValue(value string) FieldValue {
	return FieldValue{
		Type:   FieldString,
		String: value,
	}
}

func BoolValue(value bool) FieldValue {
	return FieldValue{
		Type: FieldBool,
		Bool: value,
	}
}

type Point struct {
	Database        string                `json:"database"`
	RetentionPolicy string                `json:"retention_policy"`
	Measurement     string                `json:"measurement"`
	Tags            map[string]string     `json:"tags"`
	Timestamp       int64                 `json:"timestamp"`
	Fields          map[string]FieldValue `json:"fields"`
}

type TagColumn struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type TypedFieldColumn struct {
	Name          string    `json:"name"`
	Type          FieldType `json:"type"`
	Float64Values []float64 `json:"float64_values,omitempty"`
	Int64Values   []int64   `json:"int64_values,omitempty"`
	StringValues  []string  `json:"string_values,omitempty"`
	BoolValues    []bool    `json:"bool_values,omitempty"`
}

type TypedBatch struct {
	Database        string             `json:"database"`
	RetentionPolicy string             `json:"retention_policy"`
	Measurement     string             `json:"measurement"`
	Tags            []TagColumn        `json:"tags"`
	Timestamps      []int64            `json:"timestamps"`
	Fields          []TypedFieldColumn `json:"fields"`
}

type ResolvedTypedFieldColumn struct {
	FieldID       uint32    `json:"field_id"`
	Name          string    `json:"name"`
	Type          FieldType `json:"type"`
	Float64Values []float64 `json:"float64_values,omitempty"`
	Int64Values   []int64   `json:"int64_values,omitempty"`
	StringValues  []string  `json:"string_values,omitempty"`
	BoolValues    []bool    `json:"bool_values,omitempty"`
}

type ResolvedTypedBatch struct {
	Database        string                     `json:"database"`
	RetentionPolicy string                     `json:"retention_policy"`
	Measurement     string                     `json:"measurement"`
	Tags            []TagColumn                `json:"tags"`
	Timestamps      []int64                    `json:"timestamps"`
	SeriesIDs       []uint64                   `json:"series_ids"`
	WriteSeqs       []uint64                   `json:"write_seqs"`
	Fields          []ResolvedTypedFieldColumn `json:"fields"`
}

type Query struct {
	Database        string            `json:"database"`
	RetentionPolicy string            `json:"retention_policy"`
	Measurement     string            `json:"measurement"`
	Tags            map[string]string `json:"tags"`
	Fields          []string          `json:"fields"`
	StartTime       int64             `json:"start_time"`
	EndTime         int64             `json:"end_time"`
	Aggregates      []AggregateSpec   `json:"aggregates"`
	Window          time.Duration     `json:"window"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
	Budget          QueryBudget       `json:"budget"`
}

type AggregateSpec struct {
	Field    string `json:"field"`
	Function string `json:"function"`
}

type QueryBudget struct {
	MaxShards  int `json:"max_shards"`
	MaxParts   int `json:"max_parts"`
	MaxSamples int `json:"max_samples"`
}

type QueryBoundaryMode uint8

const (
	QueryBoundaryNone QueryBoundaryMode = iota
	QueryBoundaryFirst
	QueryBoundaryLast
	QueryBoundaryBoth
)

type QueryExplain struct {
	Database        string            `json:"database"`
	RetentionPolicy string            `json:"retention_policy"`
	Measurement     string            `json:"measurement"`
	TagFilters      map[string]string `json:"tag_filters"`
	FieldFilters    []string          `json:"field_filters"`
	SeriesCount     int               `json:"series_count"`
	FieldCount      int               `json:"field_count"`
	CandidateShards int               `json:"candidate_shards"`
	MatchedShards   int               `json:"matched_shards"`
	SkippedShards   int               `json:"skipped_shards"`
	Pushdowns       []string          `json:"pushdowns"`
	Budget          QueryBudget       `json:"budget"`
}

type QueryStats struct {
	CandidateShards   int   `json:"candidate_shards"`
	ShardsScanned     int   `json:"shards_scanned"`
	ShardsSkipped     int   `json:"shards_skipped"`
	PartsScanned      int   `json:"parts_scanned"`
	PartsSkipped      int   `json:"parts_skipped"`
	IndexRowsRead     int   `json:"index_rows_read"`
	IndexRowsSkipped  int   `json:"index_rows_skipped"`
	TimeBlocksRead    int   `json:"time_blocks_read"`
	ValueBlocksRead   int   `json:"value_blocks_read"`
	ValuePagesRead    int   `json:"value_pages_read"`
	ValuePagesSkipped int   `json:"value_pages_skipped"`
	SamplesRead       int   `json:"samples_read"`
	SamplesReturned   int   `json:"samples_returned"`
	Errors            int   `json:"errors"`
	DurationNanos     int64 `json:"duration_nanos"`
	BudgetErrors      int   `json:"budget_errors"`
	Cancellations     int   `json:"cancellations"`
	StartedUnixNanos  int64 `json:"started_unix_nanos"`
}

type Options struct {
	Path                   string
	DefaultDatabase        string
	DefaultRetentionPolicy string
	ShardDuration          time.Duration
	Retention              time.Duration
	MemTableMaxSamples     int
	WAL                    WALOptions
	Compaction             CompactionOptions
	Compression            CompressionOptions
	StorageMemory          StorageMemoryOptions
}

type StorageMemoryOptions struct {
	SoftSampleLimit       int
	HardSampleLimit       int
	SoftBytesLimit        int64
	HardBytesLimit        int64
	QueryBytesLimit       int64
	FlushBytesLimit       int64
	CompactionBytesLimit  int64
	CompressionBytesLimit int64
}

type WALOptions struct {
	Sync          bool
	SegmentBytes  int64
	BatchRecords  int
	BatchBytes    int64
	BatchInterval time.Duration
}

type CompactionOptions struct {
	Enabled                    bool
	Level0PartLimit            int
	Level0SizeLimit            int64
	MaxOutputPartBytes         int64
	Levels                     []CompactionLevelOptions
	MaxCascadeSteps            int
	BackgroundInterval         time.Duration
	ReadAmplificationPartLimit int
	BacklogDegradedThreshold   int
	DiskSpaceReserveBytes      int64
	MinFreeBytes               int64
}

type CompactionLevelOptions struct {
	Level              int
	PartLimit          int
	SizeLimit          int64
	MaxOutputPartBytes int64
	Compression        CompressionOptions
}

type RetentionPolicy struct {
	Name     string
	Duration time.Duration
}

type FieldSchema struct {
	Measurement string
	Name        string
	Type        FieldType
}

type Series struct {
	ID          uint64
	Measurement string
	Tags        map[string]string
}

type CompressionOptions struct {
	Enabled       bool
	Timestamp     string
	Float         string
	Int           string
	String        string
	Algorithm     string
	MinPageValues int
}

type WriteOptions struct {
	Sync bool
}

type ResolvedField struct {
	FieldID   uint32     `json:"field_id"`
	FieldName string     `json:"field_name"`
	Type      FieldType  `json:"type"`
	Value     FieldValue `json:"value"`
}

type ResolvedPoint struct {
	Database        string            `json:"database"`
	RetentionPolicy string            `json:"retention_policy"`
	Measurement     string            `json:"measurement"`
	Tags            map[string]string `json:"tags"`
	SeriesID        uint64            `json:"series_id"`
	Timestamp       int64             `json:"timestamp"`
	WriteSeq        uint64            `json:"write_seq"`
	Fields          []ResolvedField   `json:"fields"`
}

type VersionedSample struct {
	Timestamp int64      `json:"timestamp"`
	WriteSeq  uint64     `json:"write_seq"`
	Value     FieldValue `json:"value"`
}

type Tombstone struct {
	SeriesIDs []uint64 `json:"series_ids"`
	FieldIDs  []uint32 `json:"field_ids"`
	StartTime int64    `json:"start_time"`
	EndTime   int64    `json:"end_time"`
	WriteSeq  uint64   `json:"write_seq"`
}

type ColumnData struct {
	SeriesID  uint64            `json:"series_id"`
	FieldID   uint32            `json:"field_id"`
	FieldType FieldType         `json:"field_type"`
	Samples   []VersionedSample `json:"samples"`
}

type ColumnSeries struct {
	SeriesID    uint64 `json:"series_id"`
	Measurement string `json:"measurement"`
	Tags        map[string]string
	FieldID     uint32
	FieldName   string
	FieldType   FieldType
	Timestamps  []int64
	Values      []FieldValue
}

type Row struct {
	SeriesID    uint64
	Measurement string
	Tags        map[string]string
	Timestamp   int64
	Fields      map[string]FieldValue
}

type ColumnIterator interface {
	Next() bool
	Column() ColumnSeries
	Err() error
	Close() error
}

type RowIterator interface {
	Next() bool
	Row() Row
	Err() error
	Close() error
}
