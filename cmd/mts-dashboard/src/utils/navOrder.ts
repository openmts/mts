/** 侧栏导航组内自定义排序（纯函数 + localStorage） */

export const NAV_ORDER_PREFS_KEY = 'mts.dashboard.nav-order.prefs.v1'

export type NavOrderMap = Record<string, string[]>

export function normalizeNavPath(path: string): string {
  const p = String(path || '').trim()
  if (!p) return ''
  if (!p.startsWith('/')) return `/${p}`
  return p
}

export function parseNavOrderMap(raw: unknown): NavOrderMap {
  if (!raw || typeof raw !== 'object') return {}
  const o = raw as Record<string, unknown>
  const out: NavOrderMap = {}
  for (const [section, val] of Object.entries(o)) {
    if (!Array.isArray(val)) continue
    const paths = val
      .map((x) => normalizeNavPath(String(x ?? '')))
      .filter(Boolean)
    // dedupe keep first
    const seen = new Set<string>()
    const uniq: string[] = []
    for (const p of paths) {
      if (seen.has(p)) continue
      seen.add(p)
      uniq.push(p)
    }
    if (uniq.length) out[section] = uniq
  }
  return out
}

export function loadNavOrderMap(
  storage: Pick<Storage, 'getItem'> | null,
  key = NAV_ORDER_PREFS_KEY,
): NavOrderMap {
  if (!storage) return {}
  try {
    const raw = storage.getItem(key)
    if (!raw) return {}
    return parseNavOrderMap(JSON.parse(raw))
  } catch {
    return {}
  }
}

export function saveNavOrderMap(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  map: NavOrderMap,
  key = NAV_ORDER_PREFS_KEY,
): void {
  if (!storage) return
  try {
    const clean = parseNavOrderMap(map)
    if (!Object.keys(clean).length) {
      storage.removeItem(key)
      return
    }
    storage.setItem(key, JSON.stringify({ version: 1, order: clean }))
  } catch {
    /* ignore */
  }
}

/** 兼容 {order:{}} 与扁平 map */
export function loadNavOrderPrefs(
  storage: Pick<Storage, 'getItem'> | null,
  key = NAV_ORDER_PREFS_KEY,
): NavOrderMap {
  if (!storage) return {}
  try {
    const raw = storage.getItem(key)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as unknown
    if (parsed && typeof parsed === 'object' && 'order' in (parsed as object)) {
      return parseNavOrderMap((parsed as { order?: unknown }).order)
    }
    return parseNavOrderMap(parsed)
  } catch {
    return {}
  }
}

/**
 * 按偏好重排：已知 path 按 order 优先，其余保持原相对顺序追加。
 */
export function applyNavOrder<T extends { to: string }>(
  items: readonly T[],
  order: readonly string[] | undefined,
): T[] {
  if (!order || !order.length) return items.slice()
  const byPath = new Map(items.map((x) => [x.to, x]))
  const used = new Set<string>()
  const out: T[] = []
  for (const p of order) {
    const path = normalizeNavPath(p)
    const hit = byPath.get(path)
    if (!hit || used.has(path)) continue
    out.push(hit)
    used.add(path)
  }
  for (const item of items) {
    if (used.has(item.to)) continue
    out.push(item)
  }
  return out
}

export function moveNavPath(
  order: readonly string[],
  path: string,
  direction: 'up' | 'down',
): string[] {
  const list = order.map(normalizeNavPath).filter(Boolean)
  const p = normalizeNavPath(path)
  const idx = list.indexOf(p)
  if (idx < 0) return list
  const j = direction === 'up' ? idx - 1 : idx + 1
  if (j < 0 || j >= list.length) return list
  const next = list.slice()
  const tmp = next[idx]!
  next[idx] = next[j]!
  next[j] = tmp
  return next
}

/**
 * 基于当前可见 items 与已存 order，生成完整可排序 path 列表 */
export function resolveSectionOrder(
  itemPaths: readonly string[],
  stored: readonly string[] | undefined,
): string[] {
  return applyNavOrder(
    itemPaths.map((to) => ({ to })),
    stored,
  ).map((x) => x.to)
}

export function setSectionOrder(
  map: NavOrderMap,
  sectionId: string,
  order: readonly string[],
): NavOrderMap {
  const next = { ...map }
  const cleaned = order.map(normalizeNavPath).filter(Boolean)
  if (!cleaned.length) {
    delete next[sectionId]
  } else {
    next[sectionId] = cleaned
  }
  return parseNavOrderMap(next)
}
