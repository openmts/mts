/** 最近访问路由（sessionStorage；支持固定 pin） */

export const RECENT_ROUTES_KEY = 'mts.dashboard.recent-routes.v1'
export const RECENT_ROUTES_MAX = 8
export const RECENT_PINNED_MAX = 4

export interface RecentRouteEntry {
  path: string
  name: string
  at: number
  pinned?: boolean
}

export function normalizeRecentPath(path: string): string {
  const p = String(path || '').trim() || '/'
  if (p.startsWith('/login') || p.startsWith('/force-change')) return ''
  return p
}

function normalizeEntry(raw: unknown): RecentRouteEntry | null {
  if (raw == null || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const path = normalizeRecentPath(String(o.path || ''))
  if (!path) return null
  const at = typeof o.at === 'number' ? o.at : Date.parse(String(o.at ?? ''))
  if (!Number.isFinite(at)) return null
  const pinned = o.pinned === true
  return { path, name: String(o.name || ''), at, ...(pinned ? { pinned: true } : {}) }
}

/** 固定项置顶，同组按 at 新→旧；总 cap */
export function sortRecentRoutes(
  items: readonly RecentRouteEntry[],
  max = RECENT_ROUTES_MAX,
): RecentRouteEntry[] {
  const pinned = items.filter((x) => x.pinned).sort((a, b) => b.at - a.at)
  const rest = items.filter((x) => !x.pinned).sort((a, b) => b.at - a.at)
  // 固定数量上限：超出时去掉最旧的固定项（转为普通）
  let pins = pinned.slice(0, RECENT_PINNED_MAX)
  if (pinned.length > RECENT_PINNED_MAX) {
    const demoted = pinned.slice(RECENT_PINNED_MAX).map((x) => {
      const { pinned: _p, ...restEntry } = x
      void _p
      return restEntry as RecentRouteEntry
    })
    rest.unshift(...demoted)
    rest.sort((a, b) => b.at - a.at)
  }
  return [...pins, ...rest].slice(0, max)
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
    return sortRecentRoutes(
      parsed.items
        .map((x) => normalizeEntry(x))
        .filter((x): x is RecentRouteEntry => x != null),
    )
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
  if (!path) return sortRecentRoutes(items, max)
  const prev = items.find((x) => x.path === path)
  const name = entry.name == null ? (prev?.name || '') : String(entry.name)
  const at = entry.at ?? Date.now()
  const next: RecentRouteEntry = {
    path,
    name,
    at,
    ...(prev?.pinned ? { pinned: true } : {}),
  }
  return sortRecentRoutes([next, ...items.filter((x) => x.path !== path)], max)
}

export function saveRecentRoutes(
  items: RecentRouteEntry[],
  storage: Pick<Storage, 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = RECENT_ROUTES_KEY,
): void {
  if (!storage) return
  try {
    const sorted = sortRecentRoutes(items)
    storage.setItem(key, JSON.stringify({ version: 1, items: sorted }))
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

export function togglePinRecentRoute(
  items: RecentRouteEntry[],
  path: string,
  max = RECENT_ROUTES_MAX,
): RecentRouteEntry[] {
  const p = normalizeRecentPath(path)
  if (!p) return sortRecentRoutes(items, max)
  const next = items.map((x) => {
    if (x.path !== p) return x
    if (x.pinned) {
      const { pinned: _p, ...rest } = x
      void _p
      return rest as RecentRouteEntry
    }
    return { ...x, pinned: true }
  })
  // 若固定超限，sortRecentRoutes 会降级最旧固定
  return sortRecentRoutes(next, max)
}

export function pinRecentRoute(
  path: string,
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): RecentRouteEntry[] {
  const items = togglePinRecentRoute(loadRecentRoutes(storage), path)
  // toggle may unpin if already pinned — for explicit pin, force pin
  const forced = items.map((x) =>
    x.path === normalizeRecentPath(path) ? { ...x, pinned: true } : x,
  )
  const sorted = sortRecentRoutes(forced)
  saveRecentRoutes(sorted, storage)
  return sorted
}

export function setRecentRoutePinned(
  path: string,
  pinned: boolean,
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): RecentRouteEntry[] {
  const p = normalizeRecentPath(path)
  let items = loadRecentRoutes(storage)
  items = items.map((x) => {
    if (x.path !== p) return x
    if (pinned) return { ...x, pinned: true }
    const { pinned: _p, ...rest } = x
    void _p
    return rest as RecentRouteEntry
  })
  // 若路径不在列表，固定时补一条
  if (pinned && p && !items.some((x) => x.path === p)) {
    items = [{ path: p, name: '', at: Date.now(), pinned: true }, ...items]
  }
  const sorted = sortRecentRoutes(items)
  saveRecentRoutes(sorted, storage)
  return sorted
}

/** 清空：默认仅未固定；all=true 全部 */
export function clearRecentRoutes(
  storage: Pick<Storage, 'removeItem' | 'setItem' | 'getItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  opts: { all?: boolean; key?: string } = {},
): RecentRouteEntry[] {
  const key = opts.key ?? RECENT_ROUTES_KEY
  if (!storage) return []
  if (opts.all) {
    try {
      if ('removeItem' in storage && typeof storage.removeItem === 'function') {
        storage.removeItem(key)
        return []
      }
    } catch {
      /* fall through */
    }
    try {
      storage.setItem(key, JSON.stringify({ version: 1, items: [] }))
    } catch {
      /* quota */
    }
    return []
  }
  const kept = loadRecentRoutes(storage, key).filter((x) => x.pinned)
  saveRecentRoutes(kept, storage, key)
  return kept
}
