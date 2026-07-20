/** 通用列表排序（纯函数） */

export type SortDir = 'asc' | 'desc'

export interface SortState<K extends string = string> {
  key: K | ''
  dir: SortDir
}

export function emptySortState<K extends string = string>(): SortState<K> {
  return { key: '', dir: 'asc' }
}

export function parseSortState<K extends string>(
  raw: unknown,
  allowed: readonly K[],
): SortState<K> {
  if (!raw || typeof raw !== 'object') return emptySortState()
  const o = raw as { key?: unknown; dir?: unknown }
  const key = String(o.key ?? '') as K
  if (!allowed.includes(key)) return emptySortState()
  const dir: SortDir = o.dir === 'desc' ? 'desc' : 'asc'
  return { key, dir }
}

export function cycleSortState<K extends string>(
  current: SortState<K>,
  nextKey: K,
): SortState<K> {
  if (current.key !== nextKey) return { key: nextKey, dir: 'asc' }
  if (current.dir === 'asc') return { key: nextKey, dir: 'desc' }
  return emptySortState()
}

export function compareStrings(a: string, b: string): number {
  return String(a ?? '').localeCompare(String(b ?? ''), undefined, {
    numeric: true,
    sensitivity: 'base',
  })
}

export function compareBooleans(a: boolean, b: boolean): number {
  return Number(a) - Number(b)
}

export function sortByAccessor<T>(
  items: readonly T[],
  state: SortState,
  accessors: Record<string, (item: T) => string | number | boolean | null | undefined>,
): T[] {
  if (!state.key || !accessors[state.key]) return items.slice()
  const get = accessors[state.key]!
  const mul = state.dir === 'desc' ? -1 : 1
  return items
    .map((item, index) => ({ item, index }))
    .sort((x, y) => {
      const av = get(x.item)
      const bv = get(y.item)
      let cmp = 0
      if (typeof av === 'boolean' || typeof bv === 'boolean') {
        cmp = compareBooleans(Boolean(av), Boolean(bv))
      } else if (typeof av === 'number' && typeof bv === 'number') {
        cmp = av - bv
      } else {
        cmp = compareStrings(String(av ?? ''), String(bv ?? ''))
      }
      if (cmp !== 0) return cmp * mul
      return x.index - y.index
    })
    .map((x) => x.item)
}

export function loadSortState<K extends string>(
  storage: Pick<Storage, 'getItem'> | null,
  key: string,
  allowed: readonly K[],
): SortState<K> {
  if (!storage) return emptySortState()
  try {
    const raw = storage.getItem(key)
    if (!raw) return emptySortState()
    return parseSortState(JSON.parse(raw), allowed)
  } catch {
    return emptySortState()
  }
}

export function saveSortState(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  key: string,
  state: SortState,
): void {
  if (!storage) return
  try {
    if (!state.key) {
      storage.removeItem(key)
      return
    }
    storage.setItem(key, JSON.stringify({ version: 1, key: state.key, dir: state.dir }))
  } catch {
    /* ignore */
  }
}
