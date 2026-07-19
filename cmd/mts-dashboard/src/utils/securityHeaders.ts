/** 与 mts-server applySecurityHeaders 对齐的期望响应头（契约/冒烟） */

export const EXPECTED_SECURITY_HEADERS: Record<string, string> = {
  'X-Content-Type-Options': 'nosniff',
  'X-Frame-Options': 'DENY',
  'Referrer-Policy': 'no-referrer',
  'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
  'Cross-Origin-Opener-Policy': 'same-origin',
}

export const EXPECTED_CSP_PARTS = [
  "default-src 'self'",
  "frame-ancestors 'none'",
  "object-src 'none'",
  "script-src 'self'",
] as const

export function missingSecurityHeaders(headers: Record<string, string | null | undefined>): string[] {
  const missing: string[] = []
  for (const [k, want] of Object.entries(EXPECTED_SECURITY_HEADERS)) {
    const got = headers[k] ?? headers[k.toLowerCase()]
    if (!got || String(got) !== want) missing.push(k)
  }
  return missing
}

export function cspLooksCommercial(csp: string | null | undefined): boolean {
  if (!csp) return false
  return EXPECTED_CSP_PARTS.every((p) => csp.includes(p))
}

/** 仅当服务端启用 HTTP TLS 时才会写入；边缘 HTTPS 由代理配置。 */
export const EXPECTED_HSTS = 'max-age=31536000; includeSubDomains'

export function hstsLooksCommercial(hsts: string | null | undefined, tlsEnabled: boolean): boolean {
  if (!tlsEnabled) return !hsts
  return !!hsts && hsts.includes('max-age=31536000')
}

/** 可商用后台冒烟路径（逻辑清单，供文档/契约测试） */
export const COMMERCIAL_SMOKE_PATHS = [
  { id: 'login', path: '/login', requiresAuth: false },
  { id: 'overview', path: '/', requiresAuth: true },
  { id: 'query', path: '/query', requiresAuth: true },
  { id: 'write', path: '/write', requiresAuth: true },
  { id: 'databases', path: '/databases', requiresAuth: true, admin: true },
  { id: 'operations', path: '/operations', requiresAuth: true, admin: true },
  { id: 'storage', path: '/storage', requiresAuth: true, admin: true },
  { id: 'readiness', path: '/ops/readiness', requiresAuth: true, admin: true },
] as const
