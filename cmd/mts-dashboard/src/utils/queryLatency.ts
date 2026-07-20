/** 查询耗时水线分级 */

export type LatencyLevel = 'fast' | 'ok' | 'slow' | 'critical'
export type LatencyLocale = 'zh' | 'en'

export interface LatencyThresholds {
  /** 小于等于为 fast（ms） */
  fastMs: number
  /** 小于等于为 ok（ms） */
  okMs: number
  /** 小于等于为 slow（ms），更大为 critical */
  slowMs: number
}

export const DEFAULT_LATENCY_THRESHOLDS: LatencyThresholds = {
  fastMs: 50,
  okMs: 200,
  slowMs: 1000,
}

export interface LatencyWaterline {
  level: LatencyLevel
  durationMs: number
  label: string
  /** 相对 critical 上限的 0-100 展示进度（上限 slowMs*2） */
  barPercent: number
  /** tailwind 语义色 class 片段 */
  toneClass: string
}

const LEVEL_LABELS: Record<LatencyLevel, Record<LatencyLocale, string>> = {
  fast: { zh: '快速', en: 'Fast' },
  ok: { zh: '正常', en: 'Normal' },
  slow: { zh: '偏慢', en: 'Slow' },
  critical: { zh: '很慢', en: 'Very slow' },
}

export function latencyLevelLabel(level: LatencyLevel, locale: LatencyLocale = 'zh'): string {
  return LEVEL_LABELS[level][locale === 'en' ? 'en' : 'zh']
}

export function nanosToMs(nanos: number): number {
  if (!Number.isFinite(nanos) || nanos < 0) return 0
  return nanos / 1e6
}

export function classifyLatency(
  durationMs: number,
  thresholds: LatencyThresholds = DEFAULT_LATENCY_THRESHOLDS,
  locale: LatencyLocale = 'zh',
): LatencyWaterline {
  const ms = Number.isFinite(durationMs) && durationMs > 0 ? durationMs : 0
  const cap = Math.max(thresholds.slowMs * 2, 1)
  const barPercent = Math.min(100, Math.round((ms / cap) * 100))
  const loc: LatencyLocale = locale === 'en' ? 'en' : 'zh'

  let level: LatencyLevel = 'critical'
  if (ms <= thresholds.fastMs) level = 'fast'
  else if (ms <= thresholds.okMs) level = 'ok'
  else if (ms <= thresholds.slowMs) level = 'slow'

  const toneClass =
    level === 'fast'
      ? 'bg-emerald-500'
      : level === 'ok'
        ? 'bg-sky-500'
        : level === 'slow'
          ? 'bg-amber-500'
          : 'bg-red-500'

  return {
    level,
    durationMs: ms,
    label: latencyLevelLabel(level, loc),
    barPercent,
    toneClass,
  }
}

export function latencyFromNanos(
  nanos: number,
  thresholds?: LatencyThresholds,
  locale: LatencyLocale = 'zh',
): LatencyWaterline {
  return classifyLatency(nanosToMs(nanos), thresholds, locale)
}
