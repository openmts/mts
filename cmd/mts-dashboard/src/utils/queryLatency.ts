/** 查询耗时水线分级 */

export type LatencyLevel = 'fast' | 'ok' | 'slow' | 'critical'

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

export function nanosToMs(nanos: number): number {
  if (!Number.isFinite(nanos) || nanos < 0) return 0
  return nanos / 1e6
}

export function classifyLatency(
  durationMs: number,
  thresholds: LatencyThresholds = DEFAULT_LATENCY_THRESHOLDS,
): LatencyWaterline {
  const ms = Number.isFinite(durationMs) && durationMs > 0 ? durationMs : 0
  const cap = Math.max(thresholds.slowMs * 2, 1)
  const barPercent = Math.min(100, Math.round((ms / cap) * 100))

  if (ms <= thresholds.fastMs) {
    return {
      level: 'fast',
      durationMs: ms,
      label: '快速',
      barPercent,
      toneClass: 'bg-emerald-500',
    }
  }
  if (ms <= thresholds.okMs) {
    return {
      level: 'ok',
      durationMs: ms,
      label: '正常',
      barPercent,
      toneClass: 'bg-sky-500',
    }
  }
  if (ms <= thresholds.slowMs) {
    return {
      level: 'slow',
      durationMs: ms,
      label: '偏慢',
      barPercent,
      toneClass: 'bg-amber-500',
    }
  }
  return {
    level: 'critical',
    durationMs: ms,
    label: '很慢',
    barPercent,
    toneClass: 'bg-red-500',
  }
}

export function latencyFromNanos(
  nanos: number,
  thresholds?: LatencyThresholds,
): LatencyWaterline {
  return classifyLatency(nanosToMs(nanos), thresholds)
}
