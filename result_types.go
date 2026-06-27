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
	CurrentBytes          int64  `json:"current_bytes"`
	PeakBytes             int64  `json:"peak_bytes"`
	ActiveBytes           int64  `json:"active_bytes"`
	MemTableBytes         int64  `json:"memtable_bytes"`
	WALBytes              int64  `json:"wal_bytes"`
	ReservationBytes      int64  `json:"reservation_bytes"`
	WriteBytes            int64  `json:"write_bytes"`
	FlushBytes            int64  `json:"flush_bytes"`
	QueryBytes            int64  `json:"query_bytes"`
	CompactionBytes       int64  `json:"compaction_bytes"`
	CompressionBytes      int64  `json:"compression_bytes"`
	SoftBytesLimit        int64  `json:"soft_bytes_limit"`
	HardBytesLimit        int64  `json:"hard_bytes_limit"`
	RejectedWrites        uint64 `json:"rejected_writes"`
	RejectedReservations  uint64 `json:"rejected_reservations"`
	FlushTriggered        uint64 `json:"flush_triggered"`
	QueryBytesLimit       int64  `json:"query_bytes_limit"`
	FlushBytesLimit       int64  `json:"flush_bytes_limit"`
	CompactionBytesLimit  int64  `json:"compaction_bytes_limit"`
	CompressionBytesLimit int64  `json:"compression_bytes_limit"`
	RuntimeHeapAllocBytes int64  `json:"runtime_heap_alloc_bytes"`
	RuntimeRSSBytes       int64  `json:"runtime_rss_bytes"`
	RuntimeGapBytes       int64  `json:"runtime_gap_bytes"`
}

// RetentionPolicy 描述一个本地 retention policy。
type RetentionPolicy struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
}

// FieldSchema 描述 measurement 下的字段 schema。
type FieldSchema struct {
	Measurement string    `json:"measurement"`
	Name        string    `json:"name"`
	Type        FieldType `json:"type"`
}

// Series 描述一条 series 元数据。
type Series struct {
	ID          uint64            `json:"id"`
	Measurement string            `json:"measurement"`
	Tags        map[string]string `json:"tags"`
}

// CompactionTaskStatus 表示最近一次 compaction 任务状态。
type CompactionTaskStatus struct {
	ID          string        `json:"id"`
	State       string        `json:"state"`
	Level       int           `json:"level"`
	OutputLevel int           `json:"output_level"`
	Reason      string        `json:"reason"`
	Score       float64       `json:"score"`
	StartedAt   time.Time     `json:"started_at"`
	FinishedAt  time.Time     `json:"finished_at"`
	Duration    time.Duration `json:"duration"`
	InputParts  int           `json:"input_parts"`
	OutputParts int           `json:"output_parts"`
	InputBytes  int64         `json:"input_bytes"`
	OutputBytes int64         `json:"output_bytes"`
	DroppedRows int           `json:"dropped_rows"`
	Error       string        `json:"error"`
}

// CompactionStats 表示 compaction 累计统计。
type CompactionStats struct {
	Active          int                  `json:"active"`
	Backlog         int                  `json:"backlog"`
	Skipped         int                  `json:"skipped"`
	Total           int                  `json:"total"`
	Success         int                  `json:"success"`
	Failure         int                  `json:"failure"`
	InputParts      int                  `json:"input_parts"`
	OutputParts     int                  `json:"output_parts"`
	InputBytes      int64                `json:"input_bytes"`
	OutputBytes     int64                `json:"output_bytes"`
	DroppedRows     int                  `json:"dropped_rows"`
	OverlapCount    int                  `json:"overlap_count"`
	MaxScore        float64              `json:"max_score"`
	LastReason      string               `json:"last_reason"`
	LastLevel       int                  `json:"last_level"`
	LastOutputLevel int                  `json:"last_output_level"`
	LastDuration    time.Duration        `json:"last_duration"`
	LastError       string               `json:"last_error"`
	LastSkipReason  string               `json:"last_skip_reason"`
	LastTask        CompactionTaskStatus `json:"last_task"`
	SafeDeleteParts int                  `json:"safe_delete_parts"`
}

// CompactionResult 表示一次手动 compaction 结果。
type CompactionResult struct {
	State       string               `json:"state"`
	Duration    time.Duration        `json:"duration"`
	Shards      int                  `json:"shards"`
	InputParts  int                  `json:"input_parts"`
	OutputParts int                  `json:"output_parts"`
	InputBytes  int64                `json:"input_bytes"`
	OutputBytes int64                `json:"output_bytes"`
	DroppedRows int                  `json:"dropped_rows"`
	Error       string               `json:"error"`
	LastTask    CompactionTaskStatus `json:"last_task"`
}

// HealthSnapshot 表示 Engine 健康状态。
type HealthSnapshot struct {
	Healthy bool          `json:"healthy"`
	Ready   bool          `json:"ready"`
	Reasons []string      `json:"reasons"`
	Checks  []HealthCheck `json:"checks"`
}

// HealthCheck 表示一个健康检查项。
type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// ColumnSeries 表示按列返回的一组 series 字段数据。
type ColumnSeries struct {
	SeriesID    uint64            `json:"series_id"`
	Measurement string            `json:"measurement"`
	Tags        map[string]string `json:"tags"`
	FieldID     uint32            `json:"field_id"`
	FieldName   string            `json:"field_name"`
	FieldType   FieldType         `json:"field_type"`
	Timestamps  []int64           `json:"timestamps"`
	Values      []FieldValue      `json:"values"`
}

// Row 表示按行返回的一条时序记录。
type Row struct {
	SeriesID    uint64                `json:"series_id"`
	Measurement string                `json:"measurement"`
	Tags        map[string]string     `json:"tags"`
	Timestamp   int64                 `json:"timestamp"`
	Fields      map[string]FieldValue `json:"fields"`
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
