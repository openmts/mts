/** 全局降采样告警横幅：摘要指纹 + session 级 dismiss（纯函数） */

import type { DownsampleStatusSummaryInput } from './downsampleStatusSummary.ts'
import {
  formatDownsampleStatusSummaryLine,
  normalizeDownsampleStatusSummary,
} from './downsampleStatusSummary.ts'

export const DOWNSAMPLE_HEALTH_BANNER_DISMISS_KEY = 'mts.downsample-health-banner-dismissed-fp'

export function downsampleHealthFingerprint(
  summary: DownsampleStatusSummaryInput | null | undefined,
): string {
  const s = normalizeDownsampleStatusSummary(summary)
  return `e${s.error}:l${s.lagging}:m${s.max_lag_seconds}:t${s.total}`
}

export function shouldShowDownsampleHealthBanner(opts: {
  isAdmin: boolean
  offline: boolean
  summary: DownsampleStatusSummaryInput | null | undefined
  dismissedFingerprint?: string | null
}): boolean {
  if (!opts.isAdmin || opts.offline) return false
  const s = normalizeDownsampleStatusSummary(opts.summary)
  if (s.error <= 0 && s.lagging <= 0) return false
  const fp = downsampleHealthFingerprint(s)
  const dismissed = (opts.dismissedFingerprint || '').trim()
  if (dismissed && dismissed === fp) return false
  return true
}

export function readDismissedDownsampleHealthFingerprint(
  storage: { getItem(key: string): string | null } | null | undefined,
  key: string = DOWNSAMPLE_HEALTH_BANNER_DISMISS_KEY,
): string | null {
  if (!storage) return null
  try {
    const raw = storage.getItem(key)
    if (raw == null || raw === '') return null
    return String(raw)
  } catch {
    return null
  }
}

export function writeDismissedDownsampleHealthFingerprint(
  storage: { setItem(key: string, value: string): void } | null | undefined,
  fingerprint: string,
  key: string = DOWNSAMPLE_HEALTH_BANNER_DISMISS_KEY,
): void {
  if (!storage) return
  const fp = (fingerprint || '').trim()
  if (!fp) return
  try {
    storage.setItem(key, fp)
  } catch {
    // sessionStorage 配额等：忽略
  }
}

export function clearDismissedDownsampleHealthFingerprint(
  storage: { removeItem(key: string): void } | null | undefined,
  key: string = DOWNSAMPLE_HEALTH_BANNER_DISMISS_KEY,
): void {
  if (!storage) return
  try {
    storage.removeItem(key)
  } catch {
    // ignore
  }
}

/** 横幅「复制摘要」剪贴板文本 */
export function formatDownsampleHealthBannerCopyText(
  summary: DownsampleStatusSummaryInput | null | undefined,
): string {
  const line = formatDownsampleStatusSummaryLine(summary)
  return `MTS downsample health\n${line}`
}
