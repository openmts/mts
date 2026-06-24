package model

import (
	"log/slog"
	"time"
)

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
	Predicates      []QueryPredicate  `json:"predicates"`
	Expr            QueryExpr         `json:"expr,omitempty"`
	Aggregates      []AggregateSpec   `json:"aggregates"`
	Window          time.Duration     `json:"window"`
	Group           QueryGroup        `json:"group"`
	Order           QueryOrder        `json:"order"`
	Limit           int               `json:"limit"`
	Offset          int               `json:"offset"`
	Cursor          string            `json:"cursor,omitempty"`
	Budget          QueryBudget       `json:"budget"`
}

type QueryPredicateKind uint8

const (
	QueryPredicateTimeRange QueryPredicateKind = iota + 1
	QueryPredicateTagEq
	QueryPredicateTagNe
	QueryPredicateTagExists
	QueryPredicateTagIn
	QueryPredicateFieldEq
	QueryPredicateFieldNe
	QueryPredicateFieldGT
	QueryPredicateFieldGTE
	QueryPredicateFieldLT
	QueryPredicateFieldLTE
)

type QueryPredicate struct {
	Kind         QueryPredicateKind `json:"kind"`
	Name         string             `json:"name"`
	StringValues []string           `json:"string_values,omitempty"`
	Value        FieldValue         `json:"value,omitempty"`
	Start        int64              `json:"start,omitempty"`
	End          int64              `json:"end,omitempty"`
}

type QueryExprKind uint8

const (
	QueryExprNone QueryExprKind = iota
	QueryExprPredicate
	QueryExprAnd
	QueryExprOr
	QueryExprNot
)

type QueryExpr struct {
	Kind      QueryExprKind  `json:"kind,omitempty"`
	Predicate QueryPredicate `json:"predicate,omitempty"`
	Children  []QueryExpr    `json:"children,omitempty"`
}

type QueryGroup struct {
	Tags   []string      `json:"tags,omitempty"`
	Window time.Duration `json:"window,omitempty"`
}

type QueryOrderBy uint8

const (
	QueryOrderByNone QueryOrderBy = iota
	QueryOrderByTime
)

type QuerySortDirection uint8

const (
	QuerySortAsc QuerySortDirection = iota + 1
	QuerySortDesc
)

type QueryOrder struct {
	By        QueryOrderBy       `json:"by"`
	Direction QuerySortDirection `json:"direction"`
}

type AggregateSpec struct {
	Field    string `json:"field"`
	Function string `json:"function"`
}

type DownsamplePolicy struct {
	Name               string               `json:"name"`
	SourceDatabase     string               `json:"source_database"`
	SourceRetention    string               `json:"source_retention"`
	SourceMeasurement  string               `json:"source_measurement"`
	TargetDatabase     string               `json:"target_database"`
	TargetRetention    string               `json:"target_retention"`
	TargetMeasurement  string               `json:"target_measurement"`
	Interval           time.Duration        `json:"interval"`
	Functions          []DownsampleFunction `json:"functions"`
	GroupByTags        []string             `json:"group_by_tags"`
	Delay              time.Duration        `json:"delay"`
	RefreshInterval    time.Duration        `json:"refresh_interval"`
	Lookback           time.Duration        `json:"lookback"`
	InitialStartTime   int64                `json:"initial_start_time"`
	RunTimeout         time.Duration        `json:"run_timeout"`
	BatchSize          int                  `json:"batch_size"`
	CheckpointInterval int                  `json:"checkpoint_interval"`
	PolicyTagName      string               `json:"policy_tag_name"`
	Enabled            bool                 `json:"enabled"`
}

type DownsampleFunction struct {
	Function string `json:"function"`
	Field    string `json:"field"`
	As       string `json:"as"`
}

type DownsampleWatermark struct {
	PolicyName         string `json:"policy_name"`
	CompletedUntilUnix int64  `json:"completed_until_unix"`
	LastRunUnix        int64  `json:"last_run_unix"`
	LastSuccessUnix    int64  `json:"last_success_unix"`
	LastError          string `json:"last_error"`
	AllowPolicyReplace bool   `json:"allow_policy_replace"`
}

type DownsampleReset struct {
	CompletedUntilUnix int64 `json:"completed_until_unix"`
	AllowPolicyReplace bool  `json:"allow_policy_replace"`
	CleanupTarget      bool  `json:"cleanup_target"`
	CleanupStartUnix   int64 `json:"cleanup_start_unix"`
	CleanupEndUnix     int64 `json:"cleanup_end_unix"`
}

type DownsampleRangeOptions struct {
	AdvanceWatermark bool `json:"advance_watermark"`
}

type DownsampleDropOptions struct {
	CleanupTarget    bool  `json:"cleanup_target"`
	CleanupStartUnix int64 `json:"cleanup_start_unix"`
	CleanupEndUnix   int64 `json:"cleanup_end_unix"`
}

type DownsampleDryRunResult struct {
	PolicyName       string `json:"policy_name"`
	StartUnix        int64  `json:"start_unix"`
	EndUnix          int64  `json:"end_unix"`
	Windows          int    `json:"windows"`
	RefreshWindows   int    `json:"refresh_windows"`
	AdvanceWindows   int    `json:"advance_windows"`
	PointsEstimate   int    `json:"points_estimate"`
	GroupsEstimate   int    `json:"groups_estimate"`
	SamplesEstimate  int    `json:"samples_estimate"`
	EstimateComplete bool   `json:"estimate_complete"`
	WouldAdvance     bool   `json:"would_advance"`
}

type DownsampleStats struct {
	Active            int           `json:"active"`
	Total             int           `json:"total"`
	Success           int           `json:"success"`
	Failure           int           `json:"failure"`
	WindowsProcessed  int           `json:"windows_processed"`
	PointsWritten     int           `json:"points_written"`
	LastDuration      time.Duration `json:"last_duration"`
	LastWatermarkUnix int64         `json:"last_watermark_unix"`
	LastPolicy        string        `json:"last_policy"`
	LastError         string        `json:"last_error"`
}

type DownsamplePolicyRuntimeStats struct {
	Active            int           `json:"active"`
	Total             int           `json:"total"`
	Success           int           `json:"success"`
	Failure           int           `json:"failure"`
	WindowsProcessed  int           `json:"windows_processed"`
	PointsWritten     int           `json:"points_written"`
	LastDuration      time.Duration `json:"last_duration"`
	LastWatermarkUnix int64         `json:"last_watermark_unix"`
	LastRunUnix       int64         `json:"last_run_unix"`
	LastSuccessUnix   int64         `json:"last_success_unix"`
	LastError         string        `json:"last_error"`
}

type DownsamplePolicyStatus struct {
	PolicyName         string        `json:"policy_name"`
	Enabled            bool          `json:"enabled"`
	Active             bool          `json:"active"`
	CompletedUntilUnix int64         `json:"completed_until_unix"`
	LastRunUnix        int64         `json:"last_run_unix"`
	LastSuccessUnix    int64         `json:"last_success_unix"`
	LastError          string        `json:"last_error"`
	NextRunUnix        int64         `json:"next_run_unix"`
	LagSeconds         int64         `json:"lag_seconds"`
	LastDuration       time.Duration `json:"last_duration"`
	WindowsProcessed   int           `json:"windows_processed"`
	PointsWritten      int           `json:"points_written"`
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
	ReadEpoch       int64             `json:"read_epoch"`
	TagFilters      map[string]string `json:"tag_filters"`
	FieldFilters    []string          `json:"field_filters"`
	SeriesCount     int               `json:"series_count"`
	FieldCount      int               `json:"field_count"`
	CandidateShards int               `json:"candidate_shards"`
	MatchedShards   int               `json:"matched_shards"`
	SkippedShards   int               `json:"skipped_shards"`
	Pushdowns       []string          `json:"pushdowns"`
	Budget          QueryBudget       `json:"budget"`
	Cost            QueryCost         `json:"cost"`
}

type QueryCost struct {
	SeriesCount      int    `json:"series_count"`
	FieldCount       int    `json:"field_count"`
	CandidateShards  int    `json:"candidate_shards"`
	MatchedShards    int    `json:"matched_shards"`
	Limit            int    `json:"limit"`
	Offset           int    `json:"offset"`
	WindowNanos      int64  `json:"window_nanos"`
	Ordered          bool   `json:"ordered"`
	Cursor           bool   `json:"cursor"`
	EstimatedSamples int64  `json:"estimated_samples"`
	PlanClass        string `json:"plan_class"`
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
	ReadEpoch         int64 `json:"read_epoch"`
}

type Options struct {
	Path                   string
	DefaultDatabase        string
	DefaultRetentionPolicy string
	ShardDuration          time.Duration
	Retention              time.Duration
	MemTableMaxSamples     int
	WAL                    WALOptions
	FlushSync              bool
	Compaction             CompactionOptions
	Compression            CompressionOptions
	StorageMemory          StorageMemoryOptions
	Logger                 *slog.Logger
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
