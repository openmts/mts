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
  refresh_interval?: number
  lookback?: number
  batch_size?: number
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
  last_duration?: number
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

/** GET /api/v1/admin/maintenance/errors */
export interface MaintenanceErrorsResponse {
  errors?: string[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/doctor（含 HTTP 填充的 busy/last） */
export interface DoctorResponse {
  ok: boolean
  http_tls_enabled?: boolean
  checks?: Array<{ level: string; code: string; message: string }>
  lines?: string[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/health（包装 Health + busy/last） */
export interface AdminHealthResponse {
  health: HealthSnapshot
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/version（构建信息 + busy/last） */
export interface AdminVersionResponse {
  version: string
  commit: string
  built_at: string
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

/** GET /api/v1/admin/stats/compaction */
export interface CompactionStatsResponse {
  stats: CompactionStats
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/stats/storage-memory */
export interface StorageMemoryResponse {
  snapshot: StorageMemorySnapshot
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/storage/snapshots（列表；含 busy/last） */
export interface StorageSnapshotsResponse {
  snapshots?: Array<{ name?: string; path?: string; size_bytes?: number; mod_time?: string }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/storage/data-snapshots（列表；含 busy/last） */
export interface StorageDataSnapshotsResponse {
  snapshots?: Array<{ name?: string; kind?: string; path?: string; size_bytes?: number; mod_time?: string }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/storage/export（含 busy/last） */
export interface StorageExportResponse {
  export?: Record<string, unknown>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/config/effective|config（含 busy/last） */
export interface ConfigResponse {
  config: Record<string, unknown>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/config/schema（含 busy/last） */
export interface ConfigSchemaResponse {
  fields?: Array<{ name?: string; description?: string }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/api-spec（含 busy/last） */
export interface AdminAPISpecResponse {
  version?: string
  namespaces?: unknown[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/error-codes（含 busy/last） */
export interface AdminErrorCodesResponse {
  codes?: unknown[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/audit（含 busy/last） */
export interface AdminAuditResponse {
  events?: unknown[]
  total?: number
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/downsample/policies（列表；含 busy/last） */
export interface DownsamplePoliciesResponse {
  policies?: unknown[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/downsample/statuses 摘要 */
export interface DownsampleStatusSummary {
  total: number
  enabled: number
  active: number
  error: number
  lagging: number
  max_lag_seconds: number
}

/** GET /api/v1/admin/downsample/statuses（含 busy/last） */
export interface DownsampleStatusesResponse {
  statuses?: unknown[]
  /** 过滤后汇总；summary_only=1 时仅返回摘要 */
  summary?: DownsampleStatusSummary
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/users（列表；含 busy/last） */
export interface UsersListResponse {
  users?: Array<{ name?: string; role?: string; disabled?: boolean }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/admin/databases 或 data/databases（含 busy/last） */
export interface DatabasesListResponse {
  databases?: string[]
  measurements?: string[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET retention-policies（含 busy/last） */
export interface RetentionPoliciesListResponse {
  policies?: Array<{ name?: string; duration?: number }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET /api/v1/users/{name}/database-permissions（含 busy/last） */
export interface DatabasePermissionsListResponse {
  grants?: Array<{ database?: string; permission?: string }>
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET fields（含 busy/last） */
export interface FieldsListResponse {
  fields?: unknown[]
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

/** GET series（含 busy/last） */
export interface SeriesListResponse {
  series?: unknown[]
  total?: number
  truncated?: boolean
  limit?: number
  offset?: number
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: AdminHeavyLastResult | null
}

export interface UserInfo {
  name: string
  role?: string
  roles?: string[]
}
