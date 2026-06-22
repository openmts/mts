package mts

import "time"

// QueryExplain 描述查询计划和下推决策。
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

// QueryStats 描述最近一次查询的读取和执行统计。
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

// QueryResult 表示带 explain 和 stats 的查询结果。
type QueryResult struct {
	Columns []ColumnSeries `json:"columns"`
	Explain QueryExplain   `json:"explain"`
	Stats   QueryStats     `json:"stats"`
}

// StorageMemorySnapshot 表示存储层内存快照。
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

// RetentionPolicy 描述一个本地 retention policy。
type RetentionPolicy struct {
	Name     string
	Duration time.Duration
}

// FieldSchema 描述 measurement 下的字段 schema。
type FieldSchema struct {
	Measurement string
	Name        string
	Type        FieldType
}

// Series 描述一条 series 元数据。
type Series struct {
	ID          uint64
	Measurement string
	Tags        map[string]string
}

// CompactionTaskStatus 表示最近一次 compaction 任务状态。
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

// CompactionStats 表示 compaction 累计统计。
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

// CompactionResult 表示一次手动 compaction 结果。
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

// HealthSnapshot 表示 Engine 健康状态。
type HealthSnapshot struct {
	Healthy bool
	Ready   bool
	Reasons []string
	Checks  []HealthCheck
}

// HealthCheck 表示一个健康检查项。
type HealthCheck struct {
	Name   string
	Status string
	Reason string
}

// ColumnSeries 表示按列返回的一组 series 字段数据。
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

// Row 表示按行返回的一条时序记录。
type Row struct {
	SeriesID    uint64
	Measurement string
	Tags        map[string]string
	Timestamp   int64
	Fields      map[string]FieldValue
}

// ColumnIterator 以流式方式遍历列查询结果。
type ColumnIterator interface {
	Next() bool
	Column() ColumnSeries
	Err() error
	Close() error
}

// RowIterator 以流式方式遍历行查询结果。
type RowIterator interface {
	Next() bool
	Row() Row
	Err() error
	Close() error
}
