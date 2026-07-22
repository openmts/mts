/** 降采样状态摘要（Overview / 列表 / 导出） */

export interface DownsampleStatusSummaryInput {
  total?: number
  enabled?: number
  active?: number
  error?: number
  lagging?: number
  max_lag_seconds?: number
}

export interface DownsampleStatusLike {
  policy_name?: string
  enabled?: boolean
  active?: boolean
  last_error?: string
  lag_seconds?: number
  completed_until_unix?: number
  last_run_unix?: number
  next_run_unix?: number
}

export function emptyDownsampleStatusSummary(): Required<DownsampleStatusSummaryInput> {
  return { total: 0, enabled: 0, active: 0, error: 0, lagging: 0, max_lag_seconds: 0 }
}

function nonNegInt(v: unknown): number {
  const x = Number(v)
  if (!Number.isFinite(x) || x <= 0) return 0
  return Math.floor(x)
}

export function normalizeDownsampleStatusSummary(
  raw: DownsampleStatusSummaryInput | null | undefined,
): Required<DownsampleStatusSummaryInput> {
  const e = emptyDownsampleStatusSummary()
  if (!raw || typeof raw !== 'object') return e
  return {
    total: nonNegInt(raw.total),
    enabled: nonNegInt(raw.enabled),
    active: nonNegInt(raw.active),
    error: nonNegInt(raw.error),
    lagging: nonNegInt(raw.lagging),
    max_lag_seconds: nonNegInt(raw.max_lag_seconds),
  }
}

/** 从状态行本地汇总（服务端 summary 缺失时的回退） */
export function summarizeDownsampleStatuses(
  rows: DownsampleStatusLike[] | null | undefined,
): Required<DownsampleStatusSummaryInput> {
  const list = Array.isArray(rows) ? rows : []
  const out = emptyDownsampleStatusSummary()
  out.total = list.length
  for (const st of list) {
    if (st.enabled) out.enabled += 1
    if (st.active) out.active += 1
    if (String(st.last_error || '').trim()) out.error += 1
    const lag = Number(st.lag_seconds ?? 0)
    if (Number.isFinite(lag) && lag > 0) {
      out.lagging += 1
      if (lag > out.max_lag_seconds) out.max_lag_seconds = Math.floor(lag)
    }
  }
  return out
}

export function downsampleStatusSummaryTone(
  s: Required<DownsampleStatusSummaryInput>,
): 'ok' | 'warn' | 'bad' {
  if (s.error > 0) return 'bad'
  if (s.lagging > 0 || s.active > 0) return 'warn'
  return 'ok'
}

/** 按摘要严重度生成状态表深链（error / lagging 带 health 筛选） */
export function downsampleStatusSummaryJump(
  s: Required<DownsampleStatusSummaryInput>,
  opts?: { min_lag_seconds?: number },
): string {
  if (s.error > 0) return '/downsample?health=error#downsample-status'
  if (s.lagging > 0) {
    const lag = opts?.min_lag_seconds
    if (lag != null && lag > 0) {
      return `/downsample?health=lagging&min_lag=${Math.floor(lag)}#downsample-status`
    }
    return '/downsample?health=lagging#downsample-status'
  }
  return '/downsample#downsample-status'
}

/** 固定健康筛选深链（Overview 一键按钮） */
export function downsampleStatusHealthJump(
  health: 'error' | 'active' | 'lagging' | '',
  opts?: { min_lag_seconds?: number },
): string {
  if (health === 'error') return '/downsample?health=error#downsample-status'
  if (health === 'active') return '/downsample?health=active#downsample-status'
  if (health === 'lagging') {
    const lag = opts?.min_lag_seconds
    if (lag != null && lag > 0) {
      return `/downsample?health=lagging&min_lag=${Math.floor(lag)}#downsample-status`
    }
    return '/downsample?health=lagging#downsample-status'
  }
  return '/downsample#downsample-status'
}

export function buildDownsampleStatusSummaryExport(
  summary: DownsampleStatusSummaryInput | null | undefined,
  opts?: {
    at?: Date
    filter_health?: string
    min_lag_seconds?: number
    statuses?: DownsampleStatusLike[] | null
  },
): {
  kind: 'mts.downsample.status_summary'
  version: 1
  generated_at: string
  filter_health?: string
  min_lag_seconds?: number
  summary: Required<DownsampleStatusSummaryInput>
  statuses?: Array<{
    policy_name: string
    enabled?: boolean
    active?: boolean
    last_error?: string
    lag_seconds?: number
    completed_until_unix?: number
    last_run_unix?: number
    next_run_unix?: number
  }>
} {
  const at = opts?.at ?? new Date()
  const sum = normalizeDownsampleStatusSummary(summary)
  const out: ReturnType<typeof buildDownsampleStatusSummaryExport> = {
    kind: 'mts.downsample.status_summary',
    version: 1,
    generated_at: at.toISOString(),
    summary: sum,
  }
  if (opts?.filter_health) out.filter_health = opts.filter_health
  if (opts?.min_lag_seconds != null && opts.min_lag_seconds > 0) {
    out.min_lag_seconds = Math.floor(opts.min_lag_seconds)
  }
  if (Array.isArray(opts?.statuses)) {
    out.statuses = opts!.statuses!.map((s) => ({
      policy_name: String(s.policy_name || ''),
      enabled: s.enabled,
      active: s.active,
      last_error: s.last_error || undefined,
      lag_seconds: s.lag_seconds,
      completed_until_unix: s.completed_until_unix,
      last_run_unix: s.last_run_unix,
      next_run_unix: s.next_run_unix,
    }))
  }
  return out
}

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export function downsampleStatusSummaryToCSV(
  summary: DownsampleStatusSummaryInput | null | undefined,
): string {
  const s = normalizeDownsampleStatusSummary(summary)
  const header = ['total', 'enabled', 'active', 'error', 'lagging', 'max_lag_seconds']
  const row = [s.total, s.enabled, s.active, s.error, s.lagging, s.max_lag_seconds]
    .map((c) => escapeCSV(String(c)))
    .join(',')
  return header.join(',') + '\n' + row
}

export function downsampleStatusesToCSV(
  rows: DownsampleStatusLike[] | null | undefined,
): string {
  const header = [
    'policy_name',
    'enabled',
    'active',
    'lag_seconds',
    'completed_until_unix',
    'last_run_unix',
    'next_run_unix',
    'last_error',
  ]
  const lines = [header.join(',')]
  for (const s of rows || []) {
    lines.push(
      [
        s.policy_name || '',
        s.enabled ? 'true' : 'false',
        s.active ? 'true' : 'false',
        s.lag_seconds ?? '',
        s.completed_until_unix ?? '',
        s.last_run_unix ?? '',
        s.next_run_unix ?? '',
        s.last_error || '',
      ]
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}

/** Markdown/文本单行摘要 */
export function formatDownsampleStatusSummaryLine(
  s: DownsampleStatusSummaryInput | null | undefined,
): string {
  const n = normalizeDownsampleStatusSummary(s)
  return `total: ${n.total} · enabled: ${n.enabled} · active: ${n.active} · error: ${n.error} · lagging: ${n.lagging} · max_lag_seconds: ${n.max_lag_seconds}`
}

