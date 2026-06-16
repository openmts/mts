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
	BackgroundInterval time.Duration
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
