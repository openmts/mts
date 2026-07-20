/** 登录会话 TTL 解析（秒；空=服务端默认） */

export type LoginTTLParse =
  | { ok: true; seconds?: number }
  | { ok: false; reason: 'invalid' }

/** 解析用户输入的 TTL 秒数；空白表示不传（服务端默认）。 */
export function parseLoginTTLSeconds(raw: string): LoginTTLParse {
  const s = String(raw ?? '').trim()
  if (!s) return { ok: true }
  if (!/^\d+$/.test(s)) return { ok: false, reason: 'invalid' }
  const n = Number(s)
  if (!Number.isSafeInteger(n) || n <= 0) return { ok: false, reason: 'invalid' }
  return { ok: true, seconds: n }
}
