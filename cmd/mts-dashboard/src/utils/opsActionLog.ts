/** 运维操作本地历史（sessionStorage） */

export const OPS_ACTION_LOG_KEY = 'mts.dashboard.ops-actions.v1'
export const OPS_ACTION_LOG_VERSION = 1 as const
export const OPS_ACTION_LOG_MAX = 50

export type OpsActionKind = 'flush' | 'compact' | 'retention' | 'other'
export type OpsActionStatus = 'ok' | 'error'

export interface OpsActionEntry {
  id: string
  kind: OpsActionKind
  status: OpsActionStatus
  message: string
  at: number
}

export interface OpsActionExportPayload {
  version: typeof OPS_ACTION_LOG_VERSION
  kind: 'mts.ops.actions'
  exported_at: string
  items: OpsActionEntry[]
}

export function emptyOpsActionLog(): OpsActionEntry[] {
  return []
}

export function appendOpsAction(
  items: OpsActionEntry[],
  entry: Omit<OpsActionEntry, 'id'> & { id?: string },
  max = OPS_ACTION_LOG_MAX,
): OpsActionEntry[] {
  const next: OpsActionEntry = {
    id: entry.id || `ops-${entry.at}-${entry.kind}`,
    kind: entry.kind,
    status: entry.status,
    message: String(entry.message || '').trim() || '—',
    at: entry.at,
  }
  return [next, ...items].slice(0, max)
}

export function loadOpsActionLog(
  storage: Pick<Storage, 'getItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = OPS_ACTION_LOG_KEY,
): OpsActionEntry[] {
  if (!storage) return emptyOpsActionLog()
  try {
    const raw = storage.getItem(key)
    if (!raw) return emptyOpsActionLog()
    const parsed = JSON.parse(raw) as { items?: unknown }
    if (!Array.isArray(parsed.items)) return emptyOpsActionLog()
    return parsed.items
      .map((x) => normalizeEntry(x))
      .filter((x): x is OpsActionEntry => x != null)
      .slice(0, OPS_ACTION_LOG_MAX)
  } catch {
    return emptyOpsActionLog()
  }
}

export function saveOpsActionLog(
  items: OpsActionEntry[],
  storage: Pick<Storage, 'setItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = OPS_ACTION_LOG_KEY,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify({ version: OPS_ACTION_LOG_VERSION, items: items.slice(0, OPS_ACTION_LOG_MAX) }))
  } catch {
    /* quota / private mode */
  }
}

export function clearOpsActionLog(
  storage: Pick<Storage, 'removeItem'> | null = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
  key = OPS_ACTION_LOG_KEY,
): void {
  if (!storage) return
  try {
    storage.removeItem(key)
  } catch {
    /* ignore */
  }
}

export function buildOpsActionExport(items: OpsActionEntry[], now = new Date().toISOString()): OpsActionExportPayload {
  return {
    version: OPS_ACTION_LOG_VERSION,
    kind: 'mts.ops.actions',
    exported_at: now,
    items: items.map((x) => ({ ...x })),
  }
}

function normalizeEntry(raw: unknown): OpsActionEntry | null {
  if (raw == null || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const kind = o.kind
  const status = o.status
  if (kind !== 'flush' && kind !== 'compact' && kind !== 'retention' && kind !== 'other') return null
  if (status !== 'ok' && status !== 'error') return null
  const at = typeof o.at === 'number' ? o.at : Date.parse(String(o.at ?? ''))
  if (!Number.isFinite(at)) return null
  return {
    id: String(o.id || `ops-${at}`),
    kind,
    status,
    message: String(o.message || '—'),
    at,
  }
}
