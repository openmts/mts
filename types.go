package mts

import (
	"context"
	"time"

	storageengine "codeberg.org/mts/mts/internal/engine"
	"codeberg.org/mts/mts/internal/model"
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

type Query struct {
	Database        string            `json:"database"`
	RetentionPolicy string            `json:"retention_policy"`
	Measurement     string            `json:"measurement"`
	Tags            map[string]string `json:"tags"`
	Fields          []string          `json:"fields"`
	StartTime       int64             `json:"start_time"`
	EndTime         int64             `json:"end_time"`
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
}

type WALOptions struct {
	Sync          bool
	SegmentBytes  int64
	BatchRecords  int
	BatchBytes    int64
	BatchInterval time.Duration
}

type CompactionOptions struct {
	Enabled            bool
	Level0PartLimit    int
	Level0SizeLimit    int64
	MaxOutputPartBytes int64
	Levels             []CompactionLevelOptions
	MaxCascadeSteps    int
	BackgroundInterval time.Duration
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

func toModelQuery(query Query) model.Query {
	return model.Query{
		Database:        query.Database,
		RetentionPolicy: query.RetentionPolicy,
		Measurement:     query.Measurement,
		Tags:            cloneStringMap(query.Tags),
		Fields:          append([]string(nil), query.Fields...),
		StartTime:       query.StartTime,
		EndTime:         query.EndTime,
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
		Compaction:             toModelCompactionOptions(opts.Compaction),
		Compression:            toModelCompressionOptions(opts.Compression),
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
		Enabled:            opts.Enabled,
		Level0PartLimit:    opts.Level0PartLimit,
		Level0SizeLimit:    opts.Level0SizeLimit,
		MaxOutputPartBytes: opts.MaxOutputPartBytes,
		Levels:             levels,
		MaxCascadeSteps:    opts.MaxCascadeSteps,
		BackgroundInterval: opts.BackgroundInterval,
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
