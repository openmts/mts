/** 登录默认落地页偏好（localStorage；无 redirect 时生效） */

export const LANDING_PREFS_KEY = 'mts.dashboard.landing.prefs.v1'

/** 可选落地路径（不含登录/改密） */
export const LANDING_PATH_OPTIONS: readonly string[] = [
  '/',
  '/query',
  '/write',
  '/users',
  '/access',
  '/access/grants',
  '/databases',
  '/observability/metrics',
  '/config',
  '/operations',
  '/downsample',
  '/audit',
  '/api-spec',
  '/storage',
  '/ops/readiness',
  '/about',
  '/account',
]

const ADMIN_ONLY = new Set([
  '/databases',
  '/access/grants',
  '/observability/metrics',
  '/config',
  '/operations',
  '/downsample',
  '/audit',
  '/api-spec',
  '/storage',
  '/ops/readiness',
])

export function isKnownLandingPath(path: string): boolean {
  return LANDING_PATH_OPTIONS.includes(path)
}

export function isAdminOnlyLandingPath(path: string): boolean {
  return ADMIN_ONLY.has(path)
}

export function normalizeLandingPath(raw: unknown): string {
  const p = String(raw ?? '').trim() || '/'
  if (!p.startsWith('/') || p.startsWith('//') || p.includes('://')) return '/'
  if (p.startsWith('/login') || p.startsWith('/force-change')) return '/'
  const pathOnly = p.split('?')[0]?.split('#')[0] || '/'
  if (!isKnownLandingPath(pathOnly)) return '/'
  return pathOnly
}

export function loadLandingPath(
  storage: Pick<Storage, 'getItem'> | null,
  key = LANDING_PREFS_KEY,
): string {
  if (!storage) return '/'
  try {
    const raw = storage.getItem(key)
    if (!raw) return '/'
    if (raw.startsWith('{')) {
      const o = JSON.parse(raw) as { path?: unknown }
      return normalizeLandingPath(o.path)
    }
    return normalizeLandingPath(raw)
  } catch {
    return '/'
  }
}

export function saveLandingPath(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  path: string,
  key = LANDING_PREFS_KEY,
): void {
  if (!storage) return
  try {
    const p = normalizeLandingPath(path)
    if (p === '/') {
      storage.removeItem(key)
      return
    }
    storage.setItem(key, JSON.stringify({ version: 1, path: p }))
  } catch {
    /* ignore */
  }
}

/**
 * 解析最终落地路径：显式 redirect 优先，否则用偏好；非管理员禁入 admin 路径。
 */
export function resolveLandingPath(opts: {
  redirectRaw?: unknown
  preferredPath?: string | null
  isAdmin?: boolean
  sanitizeRedirect: (raw: unknown) => string | null
}): string {
  const fromRedirect = opts.sanitizeRedirect(opts.redirectRaw)
  if (fromRedirect) return fromRedirect
  const pref = normalizeLandingPath(opts.preferredPath ?? '/')
  if (!opts.isAdmin && isAdminOnlyLandingPath(pref)) return '/'
  return pref
}
