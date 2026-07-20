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

/** 从 route.query 解析查询页预填（仅 range；不自动点「执行查询」） */
export function parseQueryPrefill(
  query: Record<string, unknown> | { [key: string]: unknown },
): { range?: PrefillTimeRange; database?: string; measurement?: string } {
  const rangeRaw = firstQueryValue(query.range)
  const out: { range?: PrefillTimeRange; database?: string; measurement?: string } = {}
  if (isPrefillTimeRange(rangeRaw)) out.range = rangeRaw
  const database = firstQueryValue(query.database ?? query.db)
  if (database) out.database = database
  const measurement = firstQueryValue(query.measurement ?? query.meas)
  if (measurement) out.measurement = measurement
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

export function buildQueryPrefillPath(opts: {
  range?: PrefillTimeRange
  database?: string
  measurement?: string
  hash?: string
}): string {
  const params = new URLSearchParams()
  if (opts.range) params.set('range', opts.range)
  if (opts.database) params.set('database', opts.database)
  if (opts.measurement) params.set('measurement', opts.measurement)
  const qs = params.toString()
  const hash = opts.hash?.startsWith('#') ? opts.hash : opts.hash ? `#${opts.hash}` : '#query-form'
  return qs ? `/query?${qs}${hash}` : `/query${hash}`
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

