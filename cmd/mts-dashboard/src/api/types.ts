/** 前后端共享的高频 DTO（手写，对齐 mts-server JSON 契约） */

export interface HealthCheck {
  name: string
  status: string
  reason?: string
}

export interface HealthSnapshot {
  healthy: boolean
  ready: boolean
  reasons?: string[]
  checks?: HealthCheck[]
}

export interface QueryStatsData {
  candidate_shards?: number
  shards_scanned?: number
  shards_skipped?: number
  parts_scanned?: number
  parts_skipped?: number
  index_rows_read?: number
  index_rows_skipped?: number
  time_blocks_read?: number
  value_blocks_read?: number
  value_pages_read?: number
  value_pages_skipped?: number
  samples_read?: number
  samples_returned?: number
  errors?: number
  duration_nanos?: number
  budget_errors?: number
  cancellations?: number
  started_unix_nanos?: number
  read_epoch?: number
}

export interface QueryResultRow {
  series_id: number
  measurement: string
  tags: Record<string, string>
  timestamp: number
  fields: Record<string, unknown>
}

export interface DownsampleFunction {
  function: string
  field: string
  as: string
}

export interface DownsamplePolicy {
  name: string
  source_database: string
  source_measurement: string
  source_retention?: string
  target_database: string
  target_measurement: string
  target_retention?: string
  interval: number
  functions: DownsampleFunction[]
  group_by_tags: string[]
  enabled: boolean
}

export interface DownsampleStatus {
  policy_name: string
  enabled?: boolean
  active?: boolean
  completed_until_unix: number
  last_run_unix: number
  last_success_unix?: number
  last_error: string
  next_run_unix?: number
  lag_seconds?: number
  windows_processed?: number
  points_written?: number
}

export interface DownsampleRunResult {
  policy_name: string
  windows_processed: number
  points_written: number
  started_unix: number
  completed_unix: number
  completed_until_unix: number
  error?: string
}

export interface DownsampleDryRunResult {
  policy_name: string
  start_unix: number
  end_unix: number
  windows: number
  refresh_windows: number
  advance_windows: number
  points_estimate: number
  groups_estimate: number
  samples_estimate: number
  estimate_complete: boolean
  would_advance: boolean
}

export interface MaintenanceStats {
  compaction_active: number
  compaction_backlog: number
  compaction_skipped: number
  compaction_failure: number
  compaction_last_skip?: string
  downsample_active: number
  downsample_inflight: number
  downsample_failure: number
  downsample_max_concurrent?: number
  maintenance_error_count: number
}

/** 最近一次管理重操作结果（ops-status / maintenance stats） */
export interface AdminHeavyLastResult {
  op?: string
  ok?: boolean
  error?: string
  started_at_unix?: number
  finished_at_unix?: number
  duration_ms?: number
}

/** GET /api/v1/admin/stats/maintenance 响应；admin_op_busy 表示服务端管理重操作（运维/快照/恢复）互斥占用中 */
export interface MaintenanceStatsResponse {
  stats: MaintenanceStats
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/ops-status 轻量互斥状态，供 Dashboard 高频轮询 */
export interface OpsStatusResponse {
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

export interface StorageMemorySnapshot {
  heap_alloc_bytes?: number
  heap_inuse_bytes?: number
  [key: string]: number | string | undefined
}

export interface CompactionStats {
  active: number
  backlog: number
  total: number
  success: number
  failure: number
  last_error: string
}

export interface UserInfo {
  name: string
  role?: string
  roles?: string[]
}
