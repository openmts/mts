/** 会话服务端校准探测节奏（纯函数） */

import type { SessionUrgency } from './sessionExpiry.ts'

export interface SessionProbeDecision {
  shouldProbe: boolean
  /** 建议下次探测间隔 ms */
  nextDelayMs: number
}

/**
 * warn/critical 时更勤地请求 /auth/session；ok 时低频；expired 不探。
 * lastProbeAgeMs: 距上次探测的时间。
 */
export function nextSessionProbe(
  urgency: SessionUrgency,
  lastProbeAgeMs: number,
  opts?: { warnEveryMs?: number; criticalEveryMs?: number; okEveryMs?: number },
): SessionProbeDecision {
  const warnEvery = opts?.warnEveryMs ?? 60_000
  const criticalEvery = opts?.criticalEveryMs ?? 20_000
  const okEvery = opts?.okEveryMs ?? 5 * 60_000
  if (urgency === 'expired' || urgency === 'unknown') {
    return { shouldProbe: false, nextDelayMs: okEvery }
  }
  let every = okEvery
  if (urgency === 'critical') every = criticalEvery
  else if (urgency === 'warn') every = warnEvery
  const age = Math.max(0, lastProbeAgeMs)
  if (age >= every) return { shouldProbe: true, nextDelayMs: every }
  return { shouldProbe: false, nextDelayMs: Math.max(1_000, every - age) }
}
