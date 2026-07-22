/** 降采样状态摘要（Overview / 列表） */

export interface DownsampleStatusSummaryInput {
  total?: number
  enabled?: number
  active?: number
  error?: number
  lagging?: number
  max_lag_seconds?: number
}

export function emptyDownsampleStatusSummary(): Required<DownsampleStatusSummaryInput> {
  return { total: 0, enabled: 0, active: 0, error: 0, lagging: 0, max_lag_seconds: 0 }
}

export function normalizeDownsampleStatusSummary(
  raw: DownsampleStatusSummaryInput | null | undefined,
): Required<DownsampleStatusSummaryInput> {
  const e = emptyDownsampleStatusSummary()
  if (!raw || typeof raw !== 'object') return e
  const n = (v: unknown) => {
    const x = Number(v)
    return Number.isFinite(x) && x > 0 ? Math.floor(x) : 0
  }
  return {
    total: n(raw.total),
    enabled: n(raw.enabled),
    active: n(raw.active),
    error: n(raw.error),
    lagging: n(raw.lagging),
    max_lag_seconds: n(raw.max_lag_seconds),
  }
}

export function downsampleStatusSummaryTone(
  s: Required<DownsampleStatusSummaryInput>,
): 'ok' | 'warn' | 'bad' {
  if (s.error > 0) return 'bad'
  if (s.lagging > 0 || s.active > 0) return 'warn'
  return 'ok'
}

export function downsampleStatusSummaryJump(s: Required<DownsampleStatusSummaryInput>): string {
  if (s.error > 0) return '/downsample#downsample-status'
  if (s.lagging > 0) return '/downsample#downsample-status'
  return '/downsample#downsample-filters'
}
