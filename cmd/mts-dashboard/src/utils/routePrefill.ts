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

