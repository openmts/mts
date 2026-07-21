import type { SessionUrgency } from './sessionExpiry.ts'

/** 会话时钟刷新间隔：临界/已过期用 1s，其它默认 15s */
export function sessionClockTickMs(
  urgency: SessionUrgency | null | undefined,
  remainingMs = 0,
  defaultMs = 15_000,
  fineMs = 1_000,
): number {
  if (urgency === 'critical' || urgency === 'expired') return fineMs
  if (urgency === 'warn' && remainingMs > 0 && remainingMs <= 60_000) return fineMs
  return defaultMs
}
