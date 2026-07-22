/** 客户端相对服务端的时钟偏差（纯函数） */

/** 默认：|skew|>=30s 视为运维需关注的偏差异常 */
export const DEFAULT_CLOCK_SKEW_WARN_SEC = 30

export type ClockSkewUrgency = 'ok' | 'warn' | 'unknown'

export interface ClockSkewView {
  hasSample: boolean
  skewSeconds: number | null
  urgency: ClockSkewUrgency
  /** 有符号秒数文案，如 +12s / -5s；无样本为空 */
  label: string
}

export function computeClockSkewSeconds(
  serverTimeUnix: number | null | undefined,
  checkedAtMs: number | null | undefined,
): number | null {
  if (typeof serverTimeUnix !== 'number' || !Number.isFinite(serverTimeUnix)) return null
  if (typeof checkedAtMs !== 'number' || !Number.isFinite(checkedAtMs)) return null
  return Math.round(checkedAtMs / 1000 - Math.floor(serverTimeUnix))
}

export function clockSkewView(
  serverTimeUnix: number | null | undefined,
  checkedAtMs: number | null | undefined,
  warnAbsSec = DEFAULT_CLOCK_SKEW_WARN_SEC,
): ClockSkewView {
  const skew = computeClockSkewSeconds(serverTimeUnix, checkedAtMs)
  if (skew == null) {
    return { hasSample: false, skewSeconds: null, urgency: 'unknown', label: '' }
  }
  const threshold = Number.isFinite(warnAbsSec) && warnAbsSec > 0 ? warnAbsSec : DEFAULT_CLOCK_SKEW_WARN_SEC
  const urgency: ClockSkewUrgency = Math.abs(skew) >= threshold ? 'warn' : 'ok'
  const sign = skew > 0 ? '+' : ''
  return {
    hasSample: true,
    skewSeconds: skew,
    urgency,
    label: `${sign}${skew}s`,
  }
}

export function shouldShowClockSkewBanner(view: ClockSkewView): boolean {
  return view.hasSample && view.urgency === 'warn'
}
