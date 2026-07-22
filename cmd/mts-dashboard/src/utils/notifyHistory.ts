/** 通知历史（sessionStorage；toast 自动消失后仍可查阅） */

import type { NotifyKind } from './notifyQueue.ts'

export const NOTIFY_HISTORY_KEY = 'mts.dashboard.notify-history.v1'
export const NOTIFY_HISTORY_MAX = 200

export interface NotifyHistoryEntry {
  id: string
  kind: NotifyKind
  message: string
  count: number
  at: number
  /** toast 快捷跳转（可选） */
  actionLabel?: string
  actionPath?: string
}

export function appendNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  entry: {
    kind: NotifyKind
    message: string
    count?: number
    at?: number
    actionLabel?: string
    actionPath?: string
  },
  max = NOTIFY_HISTORY_MAX,
): NotifyHistoryEntry[] {
  const message = String(entry.message || '').trim() || '—'
  const kind = entry.kind
  const count = Math.max(1, entry.count ?? 1)
  const at = entry.at ?? Date.now()
  const actionPath = String(entry.actionPath || '').trim()
  const actionLabel = String(entry.actionLabel || '').trim()
  const id = `${at}-${kind}-${message.slice(0, 24)}-${actionPath.slice(0, 16)}`
  const next: NotifyHistoryEntry = { id, kind, message, count, at }
  if (actionPath) {
    next.actionPath = actionPath
    next.actionLabel = actionLabel || actionPath
  }
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
  entry: {
    kind: NotifyKind
    message: string
    count?: number
    at?: number
    actionLabel?: string
    actionPath?: string
  },
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
  const actionPath = String(o.actionPath || '').trim()
  const actionLabel = String(o.actionLabel || '').trim()
  const out: NotifyHistoryEntry = { id, kind, message, count, at }
  if (actionPath) {
    out.actionPath = actionPath
    out.actionLabel = actionLabel || actionPath
  }
  return out
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
    const hay = `${x.kind} ${x.message} ${x.actionLabel || ''} ${x.actionPath || ''}`.toLowerCase()
    return hay.includes(q)
  })
}

export function parseNotifyTimeBound(raw: unknown): number | null {
  if (raw == null || raw === '') return null
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  const s = String(raw).trim()
  if (!s) return null
  const ms = Date.parse(s)
  return Number.isFinite(ms) ? ms : null
}

/** 闭区间 [sinceMs, untilMs]；缺省端不限制 */
export function filterNotifyHistoryByTime(
  items: readonly NotifyHistoryEntry[],
  opts: { sinceMs?: number | null; untilMs?: number | null } = {},
): NotifyHistoryEntry[] {
  const since = opts.sinceMs == null || !Number.isFinite(opts.sinceMs) ? null : opts.sinceMs
  const until = opts.untilMs == null || !Number.isFinite(opts.untilMs) ? null : opts.untilMs
  if (since == null && until == null) return items.slice()
  return items.filter((x) => {
    if (since != null && x.at < since) return false
    if (until != null && x.at > until) return false
    return true
  })
}

export type NotifyHistoryQuickRange = '1h' | '24h' | '7d' | '30d' | 'all'

export function notifyHistoryRangeBounds(
  range: NotifyHistoryQuickRange,
  nowMs = Date.now(),
): { sinceMs: number | null; untilMs: number | null } {
  if (range === 'all') return { sinceMs: null, untilMs: null }
  const hours = range === '1h' ? 1 : range === '24h' ? 24 : range === '7d' ? 24 * 7 : 24 * 30
  return { sinceMs: nowMs - hours * 3600_000, untilMs: nowMs }
}

export function filterNotifyHistory(
  items: readonly NotifyHistoryEntry[],
  opts: {
    kind?: NotifyHistoryKindFilter
    query?: string
    sinceMs?: number | null
    untilMs?: number | null
  } = {},
): NotifyHistoryEntry[] {
  const byKind = filterNotifyHistoryByKind(items, opts.kind ?? 'all')
  const byTime = filterNotifyHistoryByTime(byKind, {
    sinceMs: opts.sinceMs,
    untilMs: opts.untilMs,
  })
  return searchNotifyHistory(byTime, opts.query ?? '')
}
