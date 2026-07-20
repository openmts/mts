/** 仅允许站内相对路径作为登录 redirect，防止开放重定向 */
export function sanitizeRedirect(raw: unknown): string | null {
  if (typeof raw !== 'string' || !raw) return null
  if (!raw.startsWith('/')) return null
  if (raw.startsWith('//')) return null
  if (raw.includes('://')) return null
  if (raw.startsWith('/login')) return null
  if (raw.startsWith('/force-change')) return null
  return raw
}

/** 登录/改密页展示用：截断过长路径 */
export function formatRedirectLabel(path: string, max = 96): string {
  const p = path.trim()
  if (!p) return ''
  if (p.length <= max) return p
  return `${p.slice(0, Math.max(8, max - 1))}…`
}

/** 组装带 redirect 的 query（无有效路径时省略） */
export function withRedirectQuery(
  base: Record<string, string>,
  redirectRaw: unknown,
): Record<string, string> {
  const r = sanitizeRedirect(redirectRaw)
  if (!r) return { ...base }
  return { ...base, redirect: r }
}
