package mts

import (
	"context"
	"time"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
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

type DownsampleRunResult struct {
	PolicyName         string `json:"policy_name"`
	WindowsProcessed   int    `json:"windows_processed"`
	PointsWritten      int    `json:"points_written"`
	StartedUnix        int64  `json:"started_unix"`
	CompletedUnix      int64  `json:"completed_unix"`
	CompletedUntilUnix int64  `json:"completed_until_unix"`
	Error              string `json:"error"`
}

type QueryBudget struct {
	MaxShards  int `json:"max_shards"`
	MaxParts   int `json:"max_parts"`
	MaxSamples int `json:"max_samples"`
}

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

type QueryResult struct {
	Columns []ColumnSeries `json:"columns"`
	Explain QueryExplain   `json:"explain"`
	Stats   QueryStats     `json:"stats"`
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

type StorageMemorySnapshot struct {
	CurrentBytes          int64
	PeakBytes             int64
	ActiveBytes           int64
	MemTableBytes         int64
	WALBytes              int64
	ReservationBytes      int64
	WriteBytes            int64
	FlushBytes            int64
	QueryBytes            int64
	CompactionBytes       int64
	CompressionBytes      int64
	SoftBytesLimit        int64
	HardBytesLimit        int64
	RejectedWrites        uint64
	RejectedReservations  uint64
	FlushTriggered        uint64
	QueryBytesLimit       int64
	FlushBytesLimit       int64
	CompactionBytesLimit  int64
	CompressionBytesLimit int64
	RuntimeHeapAllocBytes int64
	RuntimeRSSBytes       int64
	RuntimeGapBytes       int64
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

type CompactionTaskStatus struct {
	ID          string
	State       string
	Level       int
	OutputLevel int
	Reason      string
	Score       float64
	StartedAt   time.Time
	FinishedAt  time.Time
	Duration    time.Duration
	InputParts  int
	OutputParts int
	InputBytes  int64
	OutputBytes int64
	DroppedRows int
	Error       string
}

type CompactionStats struct {
	Active          int
	Backlog         int
	Skipped         int
	Total           int
	Success         int
	Failure         int
	InputParts      int
	OutputParts     int
	InputBytes      int64
	OutputBytes     int64
	DroppedRows     int
	OverlapCount    int
	MaxScore        float64
	LastReason      string
	LastLevel       int
	LastOutputLevel int
	LastDuration    time.Duration
	LastError       string
	LastSkipReason  string
	LastTask        CompactionTaskStatus
	SafeDeleteParts int
}

type CompactionResult struct {
	State       string
	Duration    time.Duration
	Shards      int
	InputParts  int
	OutputParts int
	InputBytes  int64
	OutputBytes int64
	DroppedRows int
	Error       string
	LastTask    CompactionTaskStatus
}

type HealthSnapshot struct {
	Healthy bool
	Ready   bool
	Reasons []string
	Checks  []HealthCheck
}

type HealthCheck struct {
	Name   string
	Status string
	Reason string
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

type Engine struct {
	inner *storageengine.Engine
}

func Open(ctx context.Context, opts Options) (*Engine, error) {
	inner, err := storageengine.Open(ctx, toModelOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Engine{
		inner: inner,
	}, nil
}

func toModelFieldType(fieldType FieldType) model.FieldType {
	return model.FieldType(fieldType)
}

func fromModelFieldType(fieldType model.FieldType) FieldType {
	return FieldType(fieldType)
}

func toModelFieldValue(value FieldValue) model.FieldValue {
	return model.FieldValue{
		Type:    toModelFieldType(value.Type),
		Float64: value.Float64,
		Int64:   value.Int64,
		String:  value.String,
		Bool:    value.Bool,
	}
}

func fromModelFieldValue(value model.FieldValue) FieldValue {
	return FieldValue{
		Type:    fromModelFieldType(value.Type),
		Float64: value.Float64,
		Int64:   value.Int64,
		String:  value.String,
		Bool:    value.Bool,
	}
}

func toModelPoint(point Point) model.Point {
	fields := make(map[string]model.FieldValue, len(point.Fields))
	for name, value := range point.Fields {
		fields[name] = toModelFieldValue(value)
	}
	return model.Point{
		Database:        point.Database,
		RetentionPolicy: point.RetentionPolicy,
		Measurement:     point.Measurement,
		Tags:            cloneStringMap(point.Tags),
		Timestamp:       point.Timestamp,
		Fields:          fields,
	}
}

func toModelPoints(points []Point) []model.Point {
	out := make([]model.Point, len(points))
	for index, point := range points {
		out[index] = toModelPoint(point)
	}
	return out
}

func toModelTypedBatch(batch TypedBatch) model.TypedBatch {
	tags := make([]model.TagColumn, len(batch.Tags))
	for index, tag := range batch.Tags {
		tags[index] = model.TagColumn{
			Name:   tag.Name,
			Values: tag.Values,
		}
	}
	fields := make([]model.TypedFieldColumn, len(batch.Fields))
	for index, field := range batch.Fields {
		fields[index] = model.TypedFieldColumn{
			Name:          field.Name,
			Type:          toModelFieldType(field.Type),
			Float64Values: field.Float64Values,
			Int64Values:   field.Int64Values,
			StringValues:  field.StringValues,
			BoolValues:    field.BoolValues,
		}
	}
	return model.TypedBatch{
		Database:        batch.Database,
		RetentionPolicy: batch.RetentionPolicy,
		Measurement:     batch.Measurement,
		Tags:            tags,
		Timestamps:      batch.Timestamps,
		Fields:          fields,
	}
}

func toModelQuery(query Query) model.Query {
	return model.Query{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		Tags:            cloneStringMap(query.Tags),
		Fields:          append([]string(nil), query.Fields...),
		StartTime:       query.StartTime,
		EndTime:         query.EndTime,
		Predicates:      toModelQueryPredicates(query.Predicates),
		Expr:            toModelQueryExpr(query.Expr),
		Aggregates:      toModelAggregateSpecs(query.Aggregates),
		Window:          query.Window,
		Group: model.QueryGroup{
			Tags:   append([]string(nil), query.Group.Tags...),
			Window: query.Group.Window,
		},
		Order: model.QueryOrder{
			By:        model.QueryOrderBy(query.Order.By),
			Direction: model.QuerySortDirection(query.Order.Direction),
		},
		Limit:  query.Limit,
		Offset: query.Offset,
		Budget: toModelQueryBudget(query.Budget),
	}
}

func toModelQueryExpr(expr QueryExpr) model.QueryExpr {
	out := model.QueryExpr{
		Kind:      model.QueryExprKind(expr.Kind),
		Predicate: toModelQueryPredicate(expr.Predicate),
		Children:  make([]model.QueryExpr, 0, len(expr.Children)),
	}
	for _, child := range expr.Children {
		out.Children = append(out.Children, toModelQueryExpr(child))
	}
	return out
}

func toModelQueryPredicates(predicates []QueryPredicate) []model.QueryPredicate {
	out := make([]model.QueryPredicate, 0, len(predicates))
	for _, predicate := range predicates {
		out = append(out, toModelQueryPredicate(predicate))
	}
	return out
}

func toModelQueryPredicate(predicate QueryPredicate) model.QueryPredicate {
	return model.QueryPredicate{
		Kind:         model.QueryPredicateKind(predicate.Kind),
		Name:         predicate.Name,
		StringValues: append([]string(nil), predicate.StringValues...),
		Value:        toModelFieldValue(predicate.Value),
		Start:        predicate.Start,
		End:          predicate.End,
	}
}

func toModelAggregateSpecs(specs []AggregateSpec) []model.AggregateSpec {
	out := make([]model.AggregateSpec, len(specs))
	for index, spec := range specs {
		out[index] = model.AggregateSpec{
			Field:    spec.Field,
			Function: spec.Function,
		}
	}
	return out
}

func toModelQueryBudget(budget QueryBudget) model.QueryBudget {
	return model.QueryBudget{
		MaxShards:  budget.MaxShards,
		MaxParts:   budget.MaxParts,
		MaxSamples: budget.MaxSamples,
	}
}

func fromModelQueryBudget(budget model.QueryBudget) QueryBudget {
	return QueryBudget{
		MaxShards:  budget.MaxShards,
		MaxParts:   budget.MaxParts,
		MaxSamples: budget.MaxSamples,
	}
}

func fromModelQueryExplain(explain model.QueryExplain) QueryExplain {
	return QueryExplain{
		Database:        explain.Database,
		RetentionPolicy: explain.RetentionPolicy,
		Measurement:     explain.Measurement,
		TagFilters:      cloneStringMap(explain.TagFilters),
		FieldFilters:    append([]string(nil), explain.FieldFilters...),
		SeriesCount:     explain.SeriesCount,
		FieldCount:      explain.FieldCount,
		CandidateShards: explain.CandidateShards,
		MatchedShards:   explain.MatchedShards,
		SkippedShards:   explain.SkippedShards,
		Pushdowns:       append([]string(nil), explain.Pushdowns...),
		Budget:          fromModelQueryBudget(explain.Budget),
	}
}

func fromModelQueryStats(stats model.QueryStats) QueryStats {
	return QueryStats{
		CandidateShards:   stats.CandidateShards,
		ShardsScanned:     stats.ShardsScanned,
		ShardsSkipped:     stats.ShardsSkipped,
		PartsScanned:      stats.PartsScanned,
		PartsSkipped:      stats.PartsSkipped,
		IndexRowsRead:     stats.IndexRowsRead,
		IndexRowsSkipped:  stats.IndexRowsSkipped,
		TimeBlocksRead:    stats.TimeBlocksRead,
		ValueBlocksRead:   stats.ValueBlocksRead,
		ValuePagesRead:    stats.ValuePagesRead,
		ValuePagesSkipped: stats.ValuePagesSkipped,
		SamplesRead:       stats.SamplesRead,
		SamplesReturned:   stats.SamplesReturned,
		Errors:            stats.Errors,
		DurationNanos:     stats.DurationNanos,
		BudgetErrors:      stats.BudgetErrors,
		Cancellations:     stats.Cancellations,
	}
}

func toModelOptions(opts Options) model.Options {
	return model.Options{
		Path:                   opts.Path,
		DefaultDatabase:        opts.DefaultDatabase,
		DefaultRetentionPolicy: opts.DefaultRetentionPolicy,
		ShardDuration:          opts.ShardDuration,
		Retention:              opts.Retention,
		MemTableMaxSamples:     opts.MemTableMaxSamples,
		WAL:                    toModelWALOptions(opts.WAL),
		FlushSync:              opts.FlushSync,
		Compaction:             toModelCompactionOptions(opts.Compaction),
		Compression:            toModelCompressionOptions(opts.Compression),
		StorageMemory:          toModelStorageMemoryOptions(opts.StorageMemory),
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
		Enabled:       opts.Enabled,
		Timestamp:     opts.Timestamp,
		Float:         opts.Float,
		Int:           opts.Int,
		String:        opts.String,
		Algorithm:     opts.Algorithm,
		MinPageValues: opts.MinPageValues,
	}
}

func toModelWriteOptions(opts WriteOptions) model.WriteOptions {
	return model.WriteOptions{Sync: opts.Sync}
}

func fromStorageMemorySnapshot(snapshot storageengine.StorageMemorySnapshot) StorageMemorySnapshot {
	return StorageMemorySnapshot{
		CurrentBytes:          snapshot.CurrentBytes,
		PeakBytes:             snapshot.PeakBytes,
		ActiveBytes:           snapshot.ActiveBytes,
		MemTableBytes:         snapshot.MemTableBytes,
		WALBytes:              snapshot.WALBytes,
		ReservationBytes:      snapshot.ReservationBytes,
		WriteBytes:            snapshot.WriteBytes,
		FlushBytes:            snapshot.FlushBytes,
		QueryBytes:            snapshot.QueryBytes,
		CompactionBytes:       snapshot.CompactionBytes,
		CompressionBytes:      snapshot.CompressionBytes,
		SoftBytesLimit:        snapshot.SoftBytesLimit,
		HardBytesLimit:        snapshot.HardBytesLimit,
		RejectedWrites:        snapshot.RejectedWrites,
		RejectedReservations:  snapshot.RejectedReservations,
		FlushTriggered:        snapshot.FlushTriggered,
		QueryBytesLimit:       snapshot.QueryBytesLimit,
		FlushBytesLimit:       snapshot.FlushBytesLimit,
		CompactionBytesLimit:  snapshot.CompactionBytesLimit,
		CompressionBytesLimit: snapshot.CompressionBytesLimit,
		RuntimeHeapAllocBytes: snapshot.RuntimeHeapAllocBytes,
		RuntimeRSSBytes:       snapshot.RuntimeRSSBytes,
		RuntimeGapBytes:       snapshot.RuntimeGapBytes,
	}
}

func fromCompactionTaskStatus(status storageengine.CompactionTaskStatus) CompactionTaskStatus {
	return CompactionTaskStatus{
		ID:          status.ID,
		State:       status.State,
		Level:       status.Level,
		OutputLevel: status.OutputLevel,
		Reason:      status.Reason,
		Score:       status.Score,
		StartedAt:   status.StartedAt,
		FinishedAt:  status.FinishedAt,
		Duration:    status.Duration,
		InputParts:  status.InputParts,
		OutputParts: status.OutputParts,
		InputBytes:  status.InputBytes,
		OutputBytes: status.OutputBytes,
		DroppedRows: status.DroppedRows,
		Error:       status.Error,
	}
}

func fromCompactionStats(stats storageengine.CompactionStats) CompactionStats {
	return CompactionStats{
		Active:          stats.Active,
		Backlog:         stats.Backlog,
		Skipped:         stats.Skipped,
		Total:           stats.Total,
		Success:         stats.Success,
		Failure:         stats.Failure,
		InputParts:      stats.InputParts,
		OutputParts:     stats.OutputParts,
		InputBytes:      stats.InputBytes,
		OutputBytes:     stats.OutputBytes,
		DroppedRows:     stats.DroppedRows,
		OverlapCount:    stats.OverlapCount,
		MaxScore:        stats.MaxScore,
		LastReason:      stats.LastReason,
		LastLevel:       stats.LastLevel,
		LastOutputLevel: stats.LastOutputLevel,
		LastDuration:    stats.LastDuration,
		LastError:       stats.LastError,
		LastSkipReason:  stats.LastSkipReason,
		LastTask:        fromCompactionTaskStatus(stats.LastTask),
		SafeDeleteParts: stats.SafeDeleteParts,
	}
}

func fromCompactionResult(result storageengine.CompactionResult) CompactionResult {
	return CompactionResult{
		State:       result.State,
		Duration:    result.Duration,
		Shards:      result.Shards,
		InputParts:  result.InputParts,
		OutputParts: result.OutputParts,
		InputBytes:  result.InputBytes,
		OutputBytes: result.OutputBytes,
		DroppedRows: result.DroppedRows,
		Error:       result.Error,
		LastTask:    fromCompactionTaskStatus(result.LastTask),
	}
}

func toModelRetentionPolicy(policy RetentionPolicy) model.RetentionPolicy {
	return model.RetentionPolicy{
		Name:     policy.Name,
		Duration: policy.Duration,
	}
}

func fromModelRetentionPolicy(policy model.RetentionPolicy) RetentionPolicy {
	return RetentionPolicy{
		Name:     policy.Name,
		Duration: policy.Duration,
	}
}

func fromModelRetentionPolicies(policies []model.RetentionPolicy) []RetentionPolicy {
	out := make([]RetentionPolicy, len(policies))
	for index, policy := range policies {
		out[index] = fromModelRetentionPolicy(policy)
	}
	return out
}

func fromModelFieldSchema(field model.FieldSchema) FieldSchema {
	return FieldSchema{
		Measurement: field.Measurement,
		Name:        field.Name,
		Type:        fromModelFieldType(field.Type),
	}
}

func fromModelFieldSchemas(fields []model.FieldSchema) []FieldSchema {
	out := make([]FieldSchema, len(fields))
	for index, field := range fields {
		out[index] = fromModelFieldSchema(field)
	}
	return out
}

func fromModelSeries(series model.Series) Series {
	return Series{
		ID:          series.ID,
		Measurement: series.Measurement,
		Tags:        cloneStringMap(series.Tags),
	}
}

func fromModelSeriesList(series []model.Series) []Series {
	out := make([]Series, len(series))
	for index, item := range series {
		out[index] = fromModelSeries(item)
	}
	return out
}

func fromModelColumnSeries(column model.ColumnSeries) ColumnSeries {
	values := make([]FieldValue, len(column.Values))
	for index, value := range column.Values {
		values[index] = fromModelFieldValue(value)
	}
	return ColumnSeries{
		SeriesID:    column.SeriesID,
		Measurement: column.Measurement,
		Tags:        cloneStringMap(column.Tags),
		FieldID:     column.FieldID,
		FieldName:   column.FieldName,
		FieldType:   fromModelFieldType(column.FieldType),
		Timestamps:  append([]int64(nil), column.Timestamps...),
		Values:      values,
	}
}

func fromModelColumnSeriesList(columns []model.ColumnSeries) []ColumnSeries {
	out := make([]ColumnSeries, len(columns))
	for index, column := range columns {
		out[index] = fromModelColumnSeries(column)
	}
	return out
}

func fromModelRow(row model.Row) Row {
	fields := make(map[string]FieldValue, len(row.Fields))
	for name, value := range row.Fields {
		fields[name] = fromModelFieldValue(value)
	}
	return Row{
		SeriesID:    row.SeriesID,
		Measurement: row.Measurement,
		Tags:        cloneStringMap(row.Tags),
		Timestamp:   row.Timestamp,
		Fields:      fields,
	}
}

func fromModelRows(rows []model.Row) []Row {
	out := make([]Row, len(rows))
	for index, row := range rows {
		out[index] = fromModelRow(row)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
