/** 路由 query 只读预填（命令面板/深链；不自动执行危险写操作） */

export type PrefillTimeRange = '1h' | '24h' | '7d' | '30d'

export const PREFILL_TIME_RANGES: readonly PrefillTimeRange[] = ['1h', '24h', '7d', '30d']

export function isPrefillTimeRange(v: unknown): v is PrefillTimeRange {
  return typeof v === 'string' && (PREFILL_TIME_RANGES as readonly string[]).includes(v)
}

/** range → 毫秒 epoch 起止（查询表单 start_time/end_time 使用 ms 字符串） */
export function timeRangeToMsBounds(
  range: PrefillTimeRange,
  nowMs = Date.now(),
): { startMs: number; endMs: number } {
  const hours =
    range === '1h' ? 1 : range === '24h' ? 24 : range === '7d' ? 24 * 7 : 24 * 30
  const endMs = Math.trunc(nowMs)
  const startMs = endMs - hours * 3600_000
  return { startMs, endMs }
}

export function timeRangeToQueryFormTimes(
  range: PrefillTimeRange,
  nowMs = Date.now(),
): { start_time: string; end_time: string } {
  const b = timeRangeToMsBounds(range, nowMs)
  return { start_time: String(b.startMs), end_time: String(b.endMs) }
}

export type QueryPrefill = {
  range?: PrefillTimeRange
  start_time?: string
  end_time?: string
  database?: string
  measurement?: string
  retention_policy?: string
  fields?: string
  tags?: string
  limit?: string
  order?: string
  window?: string
  aggregates?: string
  group_tags?: string
  predicates?: string
}

/** 从 route.query 解析查询页预填（不自动点「执行查询」） */
export function parseQueryPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): QueryPrefill {
  const out: QueryPrefill = {}
  const rangeRaw = firstQueryValue(query.range)
  if (isPrefillTimeRange(rangeRaw)) out.range = rangeRaw
  const startTime = firstQueryValue(query.start_time ?? query.start)
  if (startTime && isEpochMsString(startTime)) out.start_time = startTime
  const endTime = firstQueryValue(query.end_time ?? query.end)
  if (endTime && isEpochMsString(endTime)) out.end_time = endTime
  // 同时有 range 与绝对时间时，绝对时间优先（分享可复现）
  if (out.start_time || out.end_time) delete out.range
  const database = firstQueryValue(query.database ?? query.db)
  if (database) out.database = database
  const measurement = firstQueryValue(query.measurement ?? query.meas)
  if (measurement) out.measurement = measurement
  const rp = firstQueryValue(query.retention_policy ?? query.rp)
  if (rp) out.retention_policy = rp
  const fields = firstQueryValue(query.fields)
  if (fields) out.fields = fields
  const tags = firstQueryValue(query.tags)
  if (tags) out.tags = tags
  const limit = firstQueryValue(query.limit)
  if (limit) out.limit = limit
  const order = firstQueryValue(query.order)
  if (order) out.order = order
  const window = firstQueryValue(query.window)
  if (window) out.window = window
  const aggregates = firstQueryValue(query.aggregates)
  if (aggregates) out.aggregates = aggregates
  const group_tags = firstQueryValue(query.group_tags ?? query.groupTags)
  if (group_tags) out.group_tags = group_tags
  const predicates = firstQueryValue(query.predicates)
  if (predicates) out.predicates = predicates
  return out
}

/** 从 route.query 解析审计页预填（range/action/q；不自动导出） */
export function parseAuditPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): { range?: PrefillTimeRange; action?: string; q?: string; user?: string } {
  const rangeRaw = firstQueryValue(query.range)
  const out: { range?: PrefillTimeRange; action?: string; q?: string; user?: string } = {}
  if (isPrefillTimeRange(rangeRaw)) out.range = rangeRaw
  const action = firstQueryValue(query.action)
  if (action) out.action = action
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  const user = firstQueryValue(query.user ?? query.user_name)
  if (user) out.user = user
  return out
}

export function buildQueryPrefillPath(opts: QueryPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  const start = opts.start_time?.trim()
  const end = opts.end_time?.trim()
  const hasAbs = Boolean(start && isEpochMsString(start)) || Boolean(end && isEpochMsString(end))
  if (hasAbs) {
    if (start && isEpochMsString(start)) params.set('start_time', start)
    if (end && isEpochMsString(end)) params.set('end_time', end)
  } else if (opts.range) {
    params.set('range', opts.range)
  }
  if (opts.database) params.set('database', opts.database)
  if (opts.measurement) params.set('measurement', opts.measurement)
  if (opts.retention_policy) params.set('retention_policy', opts.retention_policy)
  if (opts.fields) params.set('fields', opts.fields)
  if (opts.tags) params.set('tags', opts.tags)
  if (opts.limit) params.set('limit', opts.limit)
  if (opts.order) params.set('order', opts.order)
  if (opts.window) params.set('window', opts.window)
  if (opts.aggregates) params.set('aggregates', opts.aggregates)
  if (opts.group_tags) params.set('group_tags', opts.group_tags)
  if (opts.predicates) params.set('predicates', opts.predicates)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#query-form'
  return qs ? `/query?${qs}${hash}` : `/query${hash}`
}

/** 从表单快照构造分享预填（省略空字段；时间用显式 start/end 时不写 range） */
export function queryFormToPrefill(form: {
  database?: string
  measurement?: string
  retention_policy?: string
  fields?: string
  tags?: string
  limit?: string
  order?: string
  window?: string
  aggregates?: string
  group_tags?: string
  predicates?: string
  start_time?: string
  end_time?: string
}, opts?: { range?: PrefillTimeRange; hash?: string }): string {
  const start = form.start_time?.trim()
  const end = form.end_time?.trim()
  const absStart = start && isEpochMsString(start) ? start : undefined
  const absEnd = end && isEpochMsString(end) ? end : undefined
  return buildQueryPrefillPath({
    range: absStart || absEnd ? undefined : opts?.range,
    start_time: absStart,
    end_time: absEnd,
    database: form.database?.trim() || undefined,
    measurement: form.measurement?.trim() || undefined,
    retention_policy: form.retention_policy?.trim() || undefined,
    fields: form.fields?.trim() || undefined,
    tags: form.tags?.trim() || undefined,
    limit: form.limit?.trim() || undefined,
    order: form.order?.trim() || undefined,
    window: form.window?.trim() || undefined,
    aggregates: form.aggregates?.trim() || undefined,
    group_tags: form.group_tags?.trim() || undefined,
    predicates: form.predicates?.trim() || undefined,
    hash: opts?.hash,
  })
}

export function buildAuditPrefillPath(opts: {
  range?: PrefillTimeRange
  action?: string
  q?: string
  user?: string
  hash?: string
}): string {
  const params = new URLSearchParams()
  if (opts.range) params.set('range', opts.range)
  if (opts.action) params.set('action', opts.action)
  if (opts.q) params.set('q', opts.q)
  if (opts.user) params.set('user', opts.user)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#audit-filters'
  return qs ? `/audit?${qs}${hash}` : `/audit${hash}`
}

/** 查询表单 epoch 毫秒字符串（纯数字，合理时间窗） */
export function isEpochMsString(v: string): boolean {
  const s = v.trim()
  if (!/^-?\d{10,16}$/.test(s)) return false
  const n = Number(s)
  if (!Number.isFinite(n)) return false
  // 允许 2001-01 至 2100 附近（ms 或 sec 误传时 10 位秒也接受为合法数字，表单约定 ms）
  return Math.abs(n) >= 1_000_000_000
}

function firstQueryValue(v: unknown): string | undefined {
  if (Array.isArray(v)) {
    const x = v[0]
    return typeof x === 'string' && x.trim() ? x.trim() : undefined
  }
  if (typeof v === 'string' && v.trim()) return v.trim()
  return undefined
}

/** 从 route.query 解析写入页预填（database/measurement；不自动提交） */
export function parseWritePrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): { database?: string; measurement?: string } {
  const out: { database?: string; measurement?: string } = {}
  const database = firstQueryValue(query.database ?? query.db)
  if (database) out.database = database
  const measurement = firstQueryValue(query.measurement ?? query.meas)
  if (measurement) out.measurement = measurement
  return out
}

export function buildWritePrefillPath(opts: {
  database?: string
  measurement?: string
  hash?: string
}): string {
  const params = new URLSearchParams()
  if (opts.database) params.set('database', opts.database)
  if (opts.measurement) params.set('measurement', opts.measurement)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#write-mode-typed'
  return qs ? `/write?${qs}${hash}` : `/write${hash}`
}

/** 从写入目标构造分享预填（不包含 payload/token） */
export function writeFormToPrefill(form: {
  database?: string
  measurement?: string
}, opts?: { hash?: string }): string {
  return buildWritePrefillPath({
    database: form.database?.trim() || undefined,
    measurement: form.measurement?.trim() || undefined,
    hash: opts?.hash,
  })
}

/** 从审计筛选构造分享预填 */
export function auditFormToPrefill(form: {
  range?: PrefillTimeRange
  action?: string
  q?: string
  user?: string
}, opts?: { hash?: string }): string {
  return buildAuditPrefillPath({
    range: form.range,
    action: form.action?.trim() || undefined,
    q: form.q?.trim() || undefined,
    user: form.user?.trim() || undefined,
    hash: opts?.hash,
  })
}


export type DatabasesPrefill = {
  database?: string
  q?: string
}

/** 库页预填：database 展开详情；q 为库名筛选（不自动写操作） */
export function parseDatabasesPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): DatabasesPrefill {
  const out: DatabasesPrefill = {}
  const database = firstQueryValue(query.database ?? query.db)
  if (database) out.database = database
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  return out
}

export function buildDatabasesPrefillPath(opts: DatabasesPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.database) params.set('database', opts.database)
  if (opts.q) params.set('q', opts.q)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#databases-filter-bar'
  return qs ? `/databases?${qs}${hash}` : `/databases${hash}`
}

export function databasesFormToPrefill(form: {
  database?: string
  q?: string
}, opts?: { hash?: string }): string {
  return buildDatabasesPrefillPath({
    database: form.database?.trim() || undefined,
    q: form.q?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type UsersPrefill = {
  q?: string
  role?: string
  /** active | disabled */
  status?: string
  user?: string
}

/** 用户页预填：筛选 + 可选打开授权面板（不自动改密/删除） */
export function parseUsersPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): UsersPrefill {
  const out: UsersPrefill = {}
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  const role = firstQueryValue(query.role)
  if (role === 'admin' || role === 'user') out.role = role
  const status = firstQueryValue(query.status)
  if (status === 'active' || status === 'disabled') out.status = status
  const user = firstQueryValue(query.user ?? query.user_name)
  if (user) out.user = user
  return out
}

export function buildUsersPrefillPath(opts: UsersPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.q) params.set('q', opts.q)
  if (opts.role) params.set('role', opts.role)
  if (opts.status) params.set('status', opts.status)
  if (opts.user) params.set('user', opts.user)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#users-filter-bar'
  return qs ? `/users?${qs}${hash}` : `/users${hash}`
}

export function usersFormToPrefill(form: {
  q?: string
  role?: string
  status?: string
  user?: string
}, opts?: { hash?: string }): string {
  return buildUsersPrefillPath({
    q: form.q?.trim() || undefined,
    role: form.role?.trim() || undefined,
    status: form.status?.trim() || undefined,
    user: form.user?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type AccessPrefill = {
  role?: string
  area?: string
  q?: string
}

/** 能力矩阵预填：role/area/搜索（只读） */
export function parseAccessPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): AccessPrefill {
  const out: AccessPrefill = {}
  const role = firstQueryValue(query.role)
  if (role === 'all' || role === 'admin' || role === 'user') out.role = role
  const area = firstQueryValue(query.area)
  if (area) out.area = area
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  return out
}

export function buildAccessPrefillPath(opts: AccessPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.role) params.set('role', opts.role)
  if (opts.area) params.set('area', opts.area)
  if (opts.q) params.set('q', opts.q)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#access-matrix-filter-bar'
  return qs ? `/access?${qs}${hash}` : `/access${hash}`
}

export function accessFormToPrefill(form: {
  role?: string
  area?: string
  q?: string
}, opts?: { hash?: string }): string {
  return buildAccessPrefillPath({
    role: form.role?.trim() || undefined,
    area: form.area?.trim() || undefined,
    q: form.q?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type AccessGrantsPrefill = {
  user?: string
  database?: string
  permission?: string
  q?: string
}

/** Grants 总览预填：user/database/permission/q（只读筛选） */
export function parseAccessGrantsPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): AccessGrantsPrefill {
  const out: AccessGrantsPrefill = {}
  const user = firstQueryValue(query.user ?? query.user_name)
  if (user) out.user = user
  const database = firstQueryValue(query.database ?? query.db)
  if (database) out.database = database
  const permission = firstQueryValue(query.permission ?? query.perm)
  if (permission) out.permission = permission
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  return out
}

export function buildAccessGrantsPrefillPath(opts: AccessGrantsPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.user) params.set('user', opts.user)
  if (opts.database) params.set('database', opts.database)
  if (opts.permission) params.set('permission', opts.permission)
  if (opts.q) params.set('q', opts.q)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#access-grants-filters'
  return qs ? `/access/grants?${qs}${hash}` : `/access/grants${hash}`
}

export function accessGrantsFormToPrefill(form: {
  user?: string
  database?: string
  permission?: string
  q?: string
}, opts?: { hash?: string }): string {
  return buildAccessGrantsPrefillPath({
    user: form.user?.trim() || undefined,
    database: form.database?.trim() || undefined,
    permission: form.permission?.trim() || undefined,
    q: form.q?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type DownsamplePrefill = {
  q?: string
  enabled?: string
}

/** 降采样策略筛选预填（不自动 run/enable） */
export function parseDownsamplePrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): DownsamplePrefill {
  const out: DownsamplePrefill = {}
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  const enabled = firstQueryValue(query.enabled)
  if (enabled === 'enabled' || enabled === 'disabled') out.enabled = enabled
  return out
}

export function buildDownsamplePrefillPath(opts: DownsamplePrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.q) params.set('q', opts.q)
  if (opts.enabled) params.set('enabled', opts.enabled)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#downsample-filters'
  return qs ? `/downsample?${qs}${hash}` : `/downsample${hash}`
}

export function downsampleFormToPrefill(form: {
  q?: string
  enabled?: string
}, opts?: { hash?: string }): string {
  return buildDownsamplePrefillPath({
    q: form.q?.trim() || undefined,
    enabled: form.enabled?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type OperationsPrefill = {
  maint_q?: string
  action_kind?: string
  action_status?: string
  action_q?: string
}

/** 运维页筛选预填：维护错误搜索 + 动作日志 kind/status/q（不自动执行 flush/compact） */
export function parseOperationsPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): OperationsPrefill {
  const out: OperationsPrefill = {}
  const maint_q = firstQueryValue(query.maint_q ?? query.maint)
  if (maint_q) out.maint_q = maint_q
  const action_kind = firstQueryValue(query.action_kind ?? query.kind)
  if (action_kind === 'flush' || action_kind === 'compact' || action_kind === 'retention' || action_kind === 'other' || action_kind === 'all') {
    out.action_kind = action_kind
  }
  const action_status = firstQueryValue(query.action_status ?? query.status)
  if (action_status === 'ok' || action_status === 'error' || action_status === 'all') {
    out.action_status = action_status
  }
  const action_q = firstQueryValue(query.action_q ?? query.q)
  if (action_q) out.action_q = action_q
  return out
}

export function buildOperationsPrefillPath(opts: OperationsPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.maint_q) params.set('maint_q', opts.maint_q)
  if (opts.action_kind && opts.action_kind !== 'all') params.set('action_kind', opts.action_kind)
  if (opts.action_status && opts.action_status !== 'all') params.set('action_status', opts.action_status)
  if (opts.action_q) params.set('action_q', opts.action_q)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#ops-action-log'
  return qs ? `/operations?${qs}${hash}` : `/operations${hash}`
}

export function operationsFormToPrefill(form: {
  maint_q?: string
  action_kind?: string
  action_status?: string
  action_q?: string
}, opts?: { hash?: string }): string {
  return buildOperationsPrefillPath({
    maint_q: form.maint_q?.trim() || undefined,
    action_kind: form.action_kind?.trim() || undefined,
    action_status: form.action_status?.trim() || undefined,
    action_q: form.action_q?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type StoragePrefill = {
  section?: string
}

const STORAGE_SECTIONS = new Set(['backup-drill', 'edge-https', 'data-restore', 'snapshots'])

/** 存储页 section 深链（仅 hash；query.section 兼容） */
export function parseStoragePrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): StoragePrefill {
  const out: StoragePrefill = {}
  const fromQuery = firstQueryValue(query.section)
  if (fromQuery && STORAGE_SECTIONS.has(fromQuery)) out.section = fromQuery
  const h = (hash || '').replace(/^#/, '')
  if (!out.section && h && STORAGE_SECTIONS.has(h)) out.section = h
  return out
}

export function buildStoragePrefillPath(opts: StoragePrefill & { hash?: string }): string {
  const section = opts.section && STORAGE_SECTIONS.has(opts.section) ? opts.section : undefined
  const hashRaw = opts.hash || (section ? `#${section}` : '#backup-drill')
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  // section 仅走 hash，避免与静态资源混淆
  return `/storage${hash}`
}

export function storageFormToPrefill(form: { section?: string }, opts?: { hash?: string }): string {
  return buildStoragePrefillPath({
    section: form.section?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type ReadinessPrefill = {
  section?: string
}

const READINESS_SECTIONS = new Set([
  'export-preflight',
  'deploy-kit',
  'signoff-notes',
  'deploy-runbook-drill',
  'readiness-action',
])

/** 就绪中心区块深链（hash / section query） */
export function parseReadinessPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): ReadinessPrefill {
  const out: ReadinessPrefill = {}
  const fromQuery = firstQueryValue(query.section)
  if (fromQuery && READINESS_SECTIONS.has(fromQuery)) out.section = fromQuery
  const h = (hash || '').replace(/^#/, '')
  if (!out.section && h && READINESS_SECTIONS.has(h)) out.section = h
  return out
}

export function buildReadinessPrefillPath(opts: ReadinessPrefill & { hash?: string }): string {
  const section = opts.section && READINESS_SECTIONS.has(opts.section) ? opts.section : undefined
  const hashRaw = opts.hash || (section ? `#${section}` : '#export-preflight')
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return `/ops/readiness${hash}`
}

export function readinessFormToPrefill(form: { section?: string }, opts?: { hash?: string }): string {
  return buildReadinessPrefillPath({
    section: form.section?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type ConfigPrefill = {
  schema_q?: string
  error_q?: string
  section?: string
}

const CONFIG_SECTIONS = new Set(['config-effective', 'config-schema', 'config-error-codes'])

/** 配置页筛选/区块预填（不自动 reload） */
export function parseConfigPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): ConfigPrefill {
  const out: ConfigPrefill = {}
  const schema_q = firstQueryValue(query.schema_q ?? query.schema)
  if (schema_q) out.schema_q = schema_q
  const error_q = firstQueryValue(query.error_q ?? query.error)
  if (error_q) out.error_q = error_q
  const section = firstQueryValue(query.section)
  if (section && CONFIG_SECTIONS.has(section)) out.section = section
  const h = (hash || '').replace(/^#/, '')
  if (!out.section && h && CONFIG_SECTIONS.has(h)) out.section = h
  return out
}

export function buildConfigPrefillPath(opts: ConfigPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.schema_q) params.set('schema_q', opts.schema_q)
  if (opts.error_q) params.set('error_q', opts.error_q)
  const qs = params.toString()
  const section = opts.section && CONFIG_SECTIONS.has(opts.section) ? opts.section : undefined
  const hashRaw = opts.hash || (section ? `#${section}` : (opts.schema_q ? '#config-schema' : opts.error_q ? '#config-error-codes' : '#config-effective'))
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return qs ? `/config?${qs}${hash}` : `/config${hash}`
}

export function configFormToPrefill(form: {
  schema_q?: string
  error_q?: string
  section?: string
}, opts?: { hash?: string }): string {
  return buildConfigPrefillPath({
    schema_q: form.schema_q?.trim() || undefined,
    error_q: form.error_q?.trim() || undefined,
    section: form.section?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type MetricsPrefill = {
  q?: string
  family?: string
}

/** 指标页搜索/展开 family 预填 */
export function parseMetricsPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): MetricsPrefill {
  const out: MetricsPrefill = {}
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  const family = firstQueryValue(query.family ?? query.name)
  if (family) out.family = family
  return out
}

export function buildMetricsPrefillPath(opts: MetricsPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.q) params.set('q', opts.q)
  if (opts.family) params.set('family', opts.family)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : (opts.family ? '#metrics-detail' : '#metrics-list')
  return qs ? `/observability/metrics?${qs}${hash}` : `/observability/metrics${hash}`
}

export function metricsFormToPrefill(form: {
  q?: string
  family?: string
}, opts?: { hash?: string }): string {
  return buildMetricsPrefillPath({
    q: form.q?.trim() || undefined,
    family: form.family?.trim() || undefined,
    hash: opts?.hash,
  })
}


export type ApiSpecPrefill = {
  ns?: string
  q?: string
}

/** API 契约页命名空间/搜索预填 */
export function parseApiSpecPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): ApiSpecPrefill {
  const out: ApiSpecPrefill = {}
  const ns = firstQueryValue(query.ns ?? query.namespace)
  if (ns) out.ns = ns
  const q = firstQueryValue(query.q ?? query.filter)
  if (q) out.q = q
  return out
}

export function buildApiSpecPrefillPath(opts: ApiSpecPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.ns) params.set('ns', opts.ns)
  if (opts.q) params.set('q', opts.q)
  const qs = params.toString()
  const hashRaw = opts.hash || '#api-spec-filters'
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return qs ? `/api-spec?${qs}${hash}` : `/api-spec${hash}`
}

export function apiSpecFormToPrefill(form: {
  ns?: string
  q?: string
}, opts?: { hash?: string }): string {
  return buildApiSpecPrefillPath({
    ns: form.ns?.trim() || undefined,
    q: form.q?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type AccountPrefill = {
  landing_q?: string
}

/** 账户页落地页筛选预填（不写密码/token） */
export function parseAccountPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): AccountPrefill {
  const out: AccountPrefill = {}
  const landing_q = firstQueryValue(query.landing_q ?? query.q)
  if (landing_q) out.landing_q = landing_q
  return out
}

export function buildAccountPrefillPath(opts: AccountPrefill & { hash?: string }): string {
  const params = new URLSearchParams()
  if (opts.landing_q) params.set('landing_q', opts.landing_q)
  const qs = params.toString()
  const hashRaw = opts.hash || '#account-landing'
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return qs ? `/account?${qs}${hash}` : `/account${hash}`
}

export function accountFormToPrefill(form: {
  landing_q?: string
}, opts?: { hash?: string }): string {
  return buildAccountPrefillPath({
    landing_q: form.landing_q?.trim() || undefined,
    hash: opts?.hash,
  })
}


export type OverviewPrefill = {
  section?: string
}

const OVERVIEW_SECTIONS = new Set([
  'overview-summary',
  'overview-readiness',
  'overview-health',
  'overview-health-checks',
  'overview-doctor',
  'overview-workspace',
  'overview-maint',
])

/** 概览页区块深链 */
export function parseOverviewPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): OverviewPrefill {
  const out: OverviewPrefill = {}
  const fromQuery = firstQueryValue(query.section)
  if (fromQuery && OVERVIEW_SECTIONS.has(fromQuery)) out.section = fromQuery
  const h = (hash || '').replace(/^#/, '')
  if (!out.section && h && OVERVIEW_SECTIONS.has(h)) out.section = h
  return out
}

export function buildOverviewPrefillPath(opts: OverviewPrefill & { hash?: string }): string {
  const section = opts.section && OVERVIEW_SECTIONS.has(opts.section) ? opts.section : undefined
  const hashRaw = opts.hash || (section ? `#${section}` : '#overview-summary')
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return `/${hash}`
}

export function overviewFormToPrefill(form: { section?: string }, opts?: { hash?: string }): string {
  return buildOverviewPrefillPath({
    section: form.section?.trim() || undefined,
    hash: opts?.hash,
  })
}

export type AboutPrefill = {
  section?: string
}

const ABOUT_SECTIONS = new Set(['about-client', 'about-server'])

/** 关于页区块深链 */
export function parseAboutPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
  hash?: string,
): AboutPrefill {
  const out: AboutPrefill = {}
  const fromQuery = firstQueryValue(query.section)
  if (fromQuery && ABOUT_SECTIONS.has(fromQuery)) out.section = fromQuery
  const h = (hash || '').replace(/^#/, '')
  if (!out.section && h && ABOUT_SECTIONS.has(h)) out.section = h
  return out
}

export function buildAboutPrefillPath(opts: AboutPrefill & { hash?: string }): string {
  const section = opts.section && ABOUT_SECTIONS.has(opts.section) ? opts.section : undefined
  const hashRaw = opts.hash || (section ? `#${section}` : '#about-client')
  const hash = hashRaw.startsWith('#') ? hashRaw : `#${hashRaw}`
  return `/about${hash}`
}

export function aboutFormToPrefill(form: { section?: string }, opts?: { hash?: string }): string {
  return buildAboutPrefillPath({
    section: form.section?.trim() || undefined,
    hash: opts?.hash,
  })
}
