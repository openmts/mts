/** 仅允许站内相对路径作为登录 redirect，防止开放重定向 */
export function sanitizeRedirect(raw: unknown): string | null {
  if (typeof raw !== 'string' || !raw) return null
  if (!raw.startsWith('/')) return null
  if (raw.startsWith('//')) return null
  if (raw.includes('://')) return null
  if (raw.startsWith('/login')) return null
  return raw
}
