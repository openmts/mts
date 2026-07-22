/** Storage 演练结果会话交接（sessionStorage，供 Readiness 归档） */

export const STORAGE_DRILL_HANDOFF_KEY = 'mts.storage.drill.handoff.v1'

export type StorageDrillKind =
  | 'validate'
  | 'snapshot'
  | 'data-snapshot'
  | 'restore-drill'
  | 'export'

export interface StorageDrillEvent {
  kind: StorageDrillKind
  at: string
  path: string
  ok: boolean
  summary: string
  details?: Record<string, string | number | boolean | null>
}

export interface StorageDrillHandoff {
  version: 1
  updated_at: string
  events: StorageDrillEvent[]
}

const MAX_EVENTS = 8

export function emptyStorageDrillHandoff(at = new Date()): StorageDrillHandoff {
  return { version: 1, updated_at: at.toISOString(), events: [] }
}

export function parseStorageDrillHandoff(raw: unknown): StorageDrillHandoff | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  if (o.version !== 1) return null
  const eventsIn = Array.isArray(o.events) ? o.events : []
  const events: StorageDrillEvent[] = []
  for (const e of eventsIn) {
    if (!e || typeof e !== 'object') continue
    const x = e as Record<string, unknown>
    const kind = String(x.kind || '') as StorageDrillKind
    if (!['validate', 'snapshot', 'data-snapshot', 'restore-drill', 'export'].includes(kind)) continue
    events.push({
      kind,
      at: String(x.at || ''),
      path: String(x.path || ''),
      ok: x.ok !== false,
      summary: String(x.summary || ''),
      details: x.details && typeof x.details === 'object' ? (x.details as StorageDrillEvent['details']) : undefined,
    })
  }
  return {
    version: 1,
    updated_at: String(o.updated_at || ''),
    events,
  }
}

export function appendStorageDrillEvent(
  prev: StorageDrillHandoff | null | undefined,
  event: StorageDrillEvent,
  at = new Date(),
): StorageDrillHandoff {
  const base = prev && prev.version === 1 ? prev : emptyStorageDrillHandoff(at)
  const events = [event, ...base.events.filter((e) => e.kind !== event.kind)].slice(0, MAX_EVENTS)
  return { version: 1, updated_at: at.toISOString(), events }
}

export function loadStorageDrillHandoff(storage: Storage | null | undefined): StorageDrillHandoff | null {
  if (!storage) return null
  try {
    const raw = storage.getItem(STORAGE_DRILL_HANDOFF_KEY)
    if (!raw) return null
    return parseStorageDrillHandoff(JSON.parse(raw) as unknown)
  } catch {
    return null
  }
}

export function saveStorageDrillHandoff(
  storage: Storage | null | undefined,
  handoff: StorageDrillHandoff,
): boolean {
  if (!storage) return false
  try {
    storage.setItem(STORAGE_DRILL_HANDOFF_KEY, JSON.stringify(handoff))
    return true
  } catch {
    return false
  }
}

export function recordStorageDrillEvent(
  storage: Storage | null | undefined,
  event: StorageDrillEvent,
  at = new Date(),
): StorageDrillHandoff {
  const next = appendStorageDrillEvent(loadStorageDrillHandoff(storage), event, at)
  saveStorageDrillHandoff(storage, next)
  return next
}

export function formatStorageDrillHandoffLine(
  handoff: StorageDrillHandoff | null | undefined,
  locale: 'zh' | 'en' = 'zh',
): string {
  if (!handoff || !handoff.events.length) {
    return locale === 'en' ? 'no storage drill events in session' : '本会话暂无存储演练事件'
  }
  const parts = handoff.events.slice(0, 5).map((e) => `${e.kind}:${e.ok ? 'ok' : 'fail'}`)
  return `${parts.join(' · ')} @ ${handoff.updated_at}`
}
