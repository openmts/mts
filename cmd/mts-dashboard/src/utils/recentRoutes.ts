/** 最近访问路由（sessionStorage） */

export const RECENT_ROUTES_KEY = 'mts.dashboard.recent-routes.v1'
export const RECENT_ROUTES_MAX = 8

export interface RecentRouteEntry {
  path: string
  name: string
  at: number
}

export function normalizeRecentPath(path: string): string {
  const p = String(path || '').trim() || '/'
  if (p.startsWith('/login') || p.startsWith('/force-change')) return ''
  return p
}

export function loadRecentRoutes(
  storage: Pick<Storage, 'getItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = RECENT_ROUTES_KEY,
): RecentRouteEntry[] {
  if (!storage) return []
  try {
    const raw = storage.getItem(key)
    if (!raw) return []
    const parsed = JSON.parse(raw) as { items?: unknown }
    if (!Array.isArray(parsed.items)) return []
    return parsed.items
      .map((x) => normalizeEntry(x))
      .filter((x): x is RecentRouteEntry => x != null)
      .slice(0, RECENT_ROUTES_MAX)
  } catch {
    return []
  }
}

export function pushRecentRoute(
  items: RecentRouteEntry[],
  entry: { path: string; name?: string | symbol | null; at?: number },
  max = RECENT_ROUTES_MAX,
): RecentRouteEntry[] {
  const path = normalizeRecentPath(entry.path)
  if (!path) return items.slice(0, max)
  const name = entry.name == null ? '' : String(entry.name)
  const at = entry.at ?? Date.now()
  const next: RecentRouteEntry = { path, name, at }
  return [next, ...items.filter((x) => x.path !== path)].slice(0, max)
}

export function saveRecentRoutes(
  items: RecentRouteEntry[],
  storage: Pick<Storage, 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = RECENT_ROUTES_KEY,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify({ version: 1, items: items.slice(0, RECENT_ROUTES_MAX) }))
  } catch {
    /* quota */
  }
}

export function recordRecentRoute(
  path: string,
  name?: string | symbol | null,
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): RecentRouteEntry[] {
  const items = pushRecentRoute(loadRecentRoutes(storage), { path, name })
  saveRecentRoutes(items, storage)
  return items
}

function normalizeEntry(raw: unknown): RecentRouteEntry | null {
  if (raw == null || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const path = normalizeRecentPath(String(o.path || ''))
  if (!path) return null
  const at = typeof o.at === 'number' ? o.at : Date.parse(String(o.at ?? ''))
  if (!Number.isFinite(at)) return null
  return { path, name: String(o.name || ''), at }
}
