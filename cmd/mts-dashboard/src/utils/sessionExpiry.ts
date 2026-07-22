/** 会话过期剩余时间展示 */

export type SessionUrgency = 'ok' | 'warn' | 'critical' | 'expired' | 'unknown'
export type SessionLocale = 'zh' | 'en'

export interface SessionExpiryView {
  urgency: SessionUrgency
  remainingMs: number
  label: string
}

export function parseExpiresAt(iso: string | null | undefined): number | null {
  if (!iso) return null
  const t = Date.parse(iso)
  return Number.isNaN(t) ? null : t
}

/**
 * @param expiresAtMs token 过期时刻
 * @param nowMs 当前时间
 * @param warnMs 进入 warn 的剩余阈值（默认 10 分钟）
 * @param criticalMs 进入 critical 的剩余阈值（默认 2 分钟）
 * @param locale 过期标签语言
 */
export function sessionExpiryView(
  expiresAtMs: number | null,
  nowMs = Date.now(),
  warnMs = 10 * 60_000,
  criticalMs = 2 * 60_000,
  locale: SessionLocale = 'zh',
): SessionExpiryView {
  if (expiresAtMs == null) {
    return { urgency: 'unknown', remainingMs: 0, label: '' }
  }
  const remainingMs = expiresAtMs - nowMs
  if (remainingMs <= 0) {
    return {
      urgency: 'expired',
      remainingMs: 0,
      label: locale === 'en' ? 'Expired' : '已过期',
    }
  }
  if (remainingMs <= criticalMs) {
    return { urgency: 'critical', remainingMs, label: formatRemaining(remainingMs) }
  }
  if (remainingMs <= warnMs) {
    return { urgency: 'warn', remainingMs, label: formatRemaining(remainingMs) }
  }
  return { urgency: 'ok', remainingMs, label: formatRemaining(remainingMs) }
}

export function formatRemaining(ms: number): string {
  if (ms <= 0) return '0m'
  const totalSec = Math.ceil(ms / 1000)
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = totalSec % 60
  if (h > 0) return `${h}h${m}m`
  if (m > 0) return `${m}m`
  return `${s}s`
}

/**
 * 合并本地 expires_at 与服务端 remaining（按校验时刻推演到 now）。
 * 有服务端样本时取 min(local, serverProjected)，避免时钟偏差导致写门禁过晚。
 */
export function projectServerRemainingMs(
  serverRemainingSec: number | null | undefined,
  checkedAtMs: number | null | undefined,
  nowMs = Date.now(),
): number | null {
  if (typeof serverRemainingSec !== 'number' || !Number.isFinite(serverRemainingSec)) return null
  if (typeof checkedAtMs !== 'number' || !Number.isFinite(checkedAtMs)) return null
  const base = Math.max(0, Math.floor(serverRemainingSec)) * 1000
  const elapsed = Math.max(0, nowMs - checkedAtMs)
  return Math.max(0, base - elapsed)
}

export function effectiveSessionRemainingMs(
  localRemainingMs: number,
  serverRemainingSec: number | null | undefined,
  checkedAtMs: number | null | undefined,
  nowMs = Date.now(),
): number {
  const local = Math.max(0, localRemainingMs)
  const projected = projectServerRemainingMs(serverRemainingSec, checkedAtMs, nowMs)
  if (projected == null) return local
  return Math.min(local, projected)
}

/** 基于有效剩余毫秒构造 urgency/label（供 banner / guard） */
export function sessionViewFromRemainingMs(
  remainingMs: number,
  hasExpiry: boolean,
  warnMs = 10 * 60_000,
  criticalMs = 2 * 60_000,
  locale: SessionLocale = 'zh',
): SessionExpiryView {
  if (!hasExpiry) {
    return { urgency: 'unknown', remainingMs: 0, label: '' }
  }
  if (remainingMs <= 0) {
    return {
      urgency: 'expired',
      remainingMs: 0,
      label: locale === 'en' ? 'Expired' : '已过期',
    }
  }
  if (remainingMs <= criticalMs) {
    return { urgency: 'critical', remainingMs, label: formatRemaining(remainingMs) }
  }
  if (remainingMs <= warnMs) {
    return { urgency: 'warn', remainingMs, label: formatRemaining(remainingMs) }
  }
  return { urgency: 'ok', remainingMs, label: formatRemaining(remainingMs) }
}
