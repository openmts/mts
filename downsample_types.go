package mts

import "time"

// DownsamplePolicy 定义一个本地降采样策略。
//
// 源数据由 SourceDatabase、SourceRetention 和 SourceMeasurement 定位，目标
// rollup 由 TargetDatabase、TargetRetention 和 TargetMeasurement 定位。
// Interval 是聚合窗口宽度；Functions 定义输出字段；GroupByTags 控制保留
// 的 tag 维度。
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

// DownsampleFunction 定义一个降采样输出字段。
type DownsampleFunction struct {
	Function string `json:"function"`
	Field    string `json:"field"`
	As       string `json:"as"`
}

// DownsampleWatermark 表示降采样策略进度。
type DownsampleWatermark struct {
	PolicyName         string `json:"policy_name"`
	CompletedUntilUnix int64  `json:"completed_until_unix"`
	LastRunUnix        int64  `json:"last_run_unix"`
	LastSuccessUnix    int64  `json:"last_success_unix"`
	LastError          string `json:"last_error"`
	AllowPolicyReplace bool   `json:"allow_policy_replace"`
}

// DownsampleReset 表示重置降采样策略进度的选项。
type DownsampleReset struct {
	CompletedUntilUnix int64 `json:"completed_until_unix"`
	AllowPolicyReplace bool  `json:"allow_policy_replace"`
	CleanupTarget      bool  `json:"cleanup_target"`
	CleanupStartUnix   int64 `json:"cleanup_start_unix"`
	CleanupEndUnix     int64 `json:"cleanup_end_unix"`
}

// DownsampleRangeOptions 控制手动 range 降采样行为。
type DownsampleRangeOptions struct {
	AdvanceWatermark bool `json:"advance_watermark"`
}

// DownsampleDropOptions 控制删除降采样策略时的目标数据清理行为。
type DownsampleDropOptions struct {
	CleanupTarget    bool  `json:"cleanup_target"`
	CleanupStartUnix int64 `json:"cleanup_start_unix"`
	CleanupEndUnix   int64 `json:"cleanup_end_unix"`
}

// DownsampleDryRunResult 表示降采样 dry-run 成本估算。
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

// DownsamplePolicyStatus 表示降采样策略运行状态。
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

// DownsampleRunResult 表示一次降采样运行结果。
type DownsampleRunResult struct {
	PolicyName         string `json:"policy_name"`
	WindowsProcessed   int    `json:"windows_processed"`
	PointsWritten      int    `json:"points_written"`
	StartedUnix        int64  `json:"started_unix"`
	CompletedUnix      int64  `json:"completed_unix"`
	CompletedUntilUnix int64  `json:"completed_until_unix"`
	Error              string `json:"error"`
}
