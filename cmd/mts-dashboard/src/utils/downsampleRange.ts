/** 降采样区间操作纯函数 */

export type DownsampleRangeMode = 'repair' | 'run-range' | 'dry-run'
export type DownsampleActionMode = DownsampleRangeMode | 'run'

export interface DownsampleRangeInput {
  startUnix: number
  endUnix: number
  advanceWatermark?: boolean
}

export interface DownsampleRangeValidation {
  ok: boolean
  error?: string
}

export function defaultDownsampleRange(nowSec = Math.floor(Date.now() / 1000)): {
  startUnix: number
  endUnix: number
} {
  return { startUnix: Math.max(1, nowSec - 24 * 3600), endUnix: nowSec }
}

/** 有完成水位时：从水位前 1h 到 now；否则默认 24h */
export function suggestDownsampleRange(
  completedUntilUnix: number | undefined | null,
  nowSec = Math.floor(Date.now() / 1000),
): { startUnix: number; endUnix: number } {
  const def = defaultDownsampleRange(nowSec)
  if (completedUntilUnix && completedUntilUnix > 0) {
    return {
      startUnix: Math.max(1, Math.floor(completedUntilUnix) - 3600),
      endUnix: def.endUnix,
    }
  }
  return def
}

export function validateDownsampleRange(input: DownsampleRangeInput): DownsampleRangeValidation {
  const start = Number(input.startUnix)
  const end = Number(input.endUnix)
  if (!Number.isFinite(start) || !Number.isFinite(end)) {
    return { ok: false, error: 'invalid_range_nan' }
  }
  if (start <= 0 || end <= 0) {
    return { ok: false, error: 'invalid_range_non_positive' }
  }
  if (end <= start) {
    return { ok: false, error: 'invalid_range_order' }
  }
  if (end - start > 90 * 24 * 3600) {
    return { ok: false, error: 'invalid_range_too_wide' }
  }
  return { ok: true }
}

export function rangeErrorMessage(code: string | undefined, locale: 'zh' | 'en' = 'zh'): string {
  const zh: Record<string, string> = {
    invalid_range_nan: '时间范围必须是有效数字',
    invalid_range_non_positive: '开始/结束时间必须为正整数 Unix 秒',
    invalid_range_order: '结束时间必须大于开始时间',
    invalid_range_too_wide: '时间范围不能超过 90 天',
  }
  const en: Record<string, string> = {
    invalid_range_nan: 'Range must be valid numbers',
    invalid_range_non_positive: 'Start/end must be positive Unix seconds',
    invalid_range_order: 'End must be greater than start',
    invalid_range_too_wide: 'Range cannot exceed 90 days',
  }
  const table = locale === 'en' ? en : zh
  return table[code || ''] || (locale === 'en' ? 'Invalid range' : '时间范围无效')
}

/** 对齐服务端 downsampleRangeRequest JSON */
export function buildDownsampleRangeBody(input: DownsampleRangeInput): {
  start_unix: number
  end_unix: number
  options: { advance_watermark: boolean }
} {
  return {
    start_unix: Math.floor(Number(input.startUnix)),
    end_unix: Math.floor(Number(input.endUnix)),
    options: { advance_watermark: !!input.advanceWatermark },
  }
}

export function rangeActionPath(policyName: string, mode: DownsampleRangeMode): string {
  return `/api/v1/admin/downsample/policies/${encodeURIComponent(policyName)}/${mode}`
}

export function formatRunResultMessage(
  mode: DownsampleActionMode,
  name: string,
  result?: {
    windows_processed?: number
    points_written?: number
    windows?: number
    points_estimate?: number
    samples_estimate?: number
    estimate_complete?: boolean
  } | null,
): string {
  if (mode === 'dry-run') {
    return `dry-run ${name}: windows=${result?.windows ?? 0} points≈${result?.points_estimate ?? 0} samples≈${result?.samples_estimate ?? 0} complete=${result?.estimate_complete}`
  }
  return `${mode} ${name}: windows=${result?.windows_processed ?? 0} points=${result?.points_written ?? 0}`
}
