/** 通知历史（sessionStorage；toast 自动消失后仍可查阅） */

import type { NotifyKind } from './notifyQueue.ts'

export const NOTIFY_HISTORY_KEY = 'mts.dashboard.notify-history.v1'
export const NOTIFY_HISTORY_MAX = 40

export interface NotifyHistoryEntry {
  id: string
  kind: NotifyKind
  message: string
  count: number
  at: number
}

export function appendNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  entry: { kind: NotifyKind; message: string; count?: number; at?: number },
  max = NOTIFY_HISTORY_MAX,
): NotifyHistoryEntry[] {
  const message = String(entry.message || '').trim() || '—'
  const kind = entry.kind
  const count = Math.max(1, entry.count ?? 1)
  const at = entry.at ?? Date.now()
  const id = `${at}-${kind}-${message.slice(0, 24)}`
  const next: NotifyHistoryEntry = { id, kind, message, count, at }
  return [next, ...items].slice(0, max)
}

export function loadNotifyHistory(
  storage: Pick<Storage, 'getItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = NOTIFY_HISTORY_KEY,
): NotifyHistoryEntry[] {
  if (!storage) return []
  try {
    const raw = storage.getItem(key)
    if (!raw) return []
    const parsed = JSON.parse(raw) as { items?: unknown }
    if (!Array.isArray(parsed.items)) return []
    return parsed.items
      .map((x) => normalize(x))
      .filter((x): x is NotifyHistoryEntry => x != null)
      .slice(0, NOTIFY_HISTORY_MAX)
  } catch {
    return []
  }
}

export function saveNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  storage: Pick<Storage, 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = NOTIFY_HISTORY_KEY,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify({ version: 1, items: items.slice(0, NOTIFY_HISTORY_MAX) }))
  } catch {
    /* quota */
  }
}

export function recordNotifyHistory(
  entry: { kind: NotifyKind; message: string; count?: number; at?: number },
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): NotifyHistoryEntry[] {
  const items = appendNotifyHistory(loadNotifyHistory(storage), entry)
  saveNotifyHistory(items, storage)
  return items
}

export function clearNotifyHistory(
  storage: Pick<Storage, 'removeItem' | 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = NOTIFY_HISTORY_KEY,
): void {
  if (!storage) return
  try {
    if ('removeItem' in storage && typeof storage.removeItem === 'function') {
      storage.removeItem(key)
      return
    }
  } catch {
    /* fall */
  }
  try {
    storage.setItem(key, JSON.stringify({ version: 1, items: [] }))
  } catch {
    /* quota */
  }
}

function normalize(raw: unknown): NotifyHistoryEntry | null {
  if (raw == null || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const kind = o.kind
  if (kind !== 'success' && kind !== 'error' && kind !== 'info' && kind !== 'warn') return null
  const message = String(o.message || '').trim()
  if (!message) return null
  const at = typeof o.at === 'number' ? o.at : Date.parse(String(o.at ?? ''))
  if (!Number.isFinite(at)) return null
  const count = typeof o.count === 'number' && o.count > 0 ? Math.floor(o.count) : 1
  const id = String(o.id || `${at}-${kind}`)
  return { id, kind, message, count, at }
}

export type NotifyHistoryKindFilter = 'all' | NotifyKind

export function filterNotifyHistoryByKind(
  items: readonly NotifyHistoryEntry[],
  kind: NotifyHistoryKindFilter,
): NotifyHistoryEntry[] {
  if (kind === 'all') return items.slice()
  return items.filter((x) => x.kind === kind)
}

export function searchNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  query: string,
): NotifyHistoryEntry[] {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return items.slice()
  return items.filter((x) => {
    const hay = `${x.kind} ${x.message}`.toLowerCase()
    return hay.includes(q)
  })
}

export function filterNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  opts: { kind?: NotifyHistoryKindFilter; query?: string } = {},
): NotifyHistoryEntry[] {
  const byKind = filterNotifyHistoryByKind(items, opts.kind ?? 'all')
  return searchNotifyHistory(byKind, opts.query ?? '')
}
