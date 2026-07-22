/** 登录会话 TTL 解析（秒；空=服务端默认） */

import { getMaxAuthTTLSeconds } from './passwordPolicy.ts'

export type LoginTTLParse =
  | { ok: true; seconds?: number }
  | { ok: false; reason: 'invalid' | 'too_large' }

/** 解析用户输入的 TTL 秒数；空白表示不传（服务端默认）。 */
export function parseLoginTTLSeconds(
  raw: string,
  opts?: { maxSeconds?: number },
): LoginTTLParse {
  const s = String(raw ?? '').trim()
  if (!s) return { ok: true }
  if (!/^\d+$/.test(s)) return { ok: false, reason: 'invalid' }
  const n = Number(s)
  if (!Number.isSafeInteger(n) || n <= 0) return { ok: false, reason: 'invalid' }
  const max = opts?.maxSeconds ?? getMaxAuthTTLSeconds()
  if (typeof max === 'number' && max > 0 && n > max) {
    return { ok: false, reason: 'too_large' }
  }
  return { ok: true, seconds: n }
}

/** 人类可读 TTL 上限文案 */
export function formatAuthTTLLimit(seconds: number): string {
  const n = Math.max(0, Math.floor(seconds || 0))
  if (n <= 0) return '—'
  const d = Math.floor(n / 86400)
  const h = Math.floor((n % 86400) / 3600)
  if (d > 0 && h > 0) return `${d}d${h}h`
  if (d > 0) return `${d}d`
  if (h > 0) return `${h}h`
  const m = Math.floor(n / 60)
  if (m > 0) return `${m}m`
  return `${n}s`
}
