/** Audit 页会话摘要：路径、筛选态、事件计数（纯函数） */

export type AuditListSource = 'admin' | 'self' | ''

export interface AuditSessionSummary {
  list_path: string
  source: AuditListSource
  event_count: number
  filtered_count: number
  server_total: number | null
  selected_user: string
  action_filter: string
  client_query: string
  has_time_range: boolean
  limit: number
  preferred_list_path: string
  path_ok: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export function preferredAuditListPath(input: {
  source?: string | null
  userName?: string | null
}): string {
  const source = String(input.source || '').trim()
  if (source === 'self') {
    const name = String(input.userName || '').trim()
    if (name) return `/api/v1/users/${encodeURIComponent(name)}/audit`
    return '/api/v1/users/{name}/audit'
  }
  return '/api/v1/admin/audit'
}

export function buildAuditSessionSummary(input: {
  listPath?: string | null
  source?: string | null
  eventCount?: number
  filteredCount?: number
  serverTotal?: number | null
  selectedUser?: string | null
  actionFilter?: string | null
  clientQuery?: string | null
  hasTimeRange?: boolean
  limit?: number
  userName?: string | null
}): AuditSessionSummary {
  const sourceRaw = String(input.source || '').trim()
  const source: AuditListSource =
    sourceRaw === 'admin' || sourceRaw === 'self' ? sourceRaw : ''
  const selected_user = String(input.selectedUser || '').trim()
  const preferred = preferredAuditListPath({
    source: source || (selected_user && !sourceRaw ? 'self' : 'admin'),
    userName: input.userName || selected_user,
  })
  const list_path = String(input.listPath || '').trim() || preferred
  const event_count = finiteNonNeg(input.eventCount)
  const filtered_count = finiteNonNeg(input.filteredCount)
  const server_total =
    input.serverTotal == null || !Number.isFinite(Number(input.serverTotal))
      ? null
      : Math.max(0, Math.trunc(Number(input.serverTotal)))
  const action_filter = String(input.actionFilter || '').trim()
  const client_query = String(input.clientQuery || '').trim()
  const has_time_range = Boolean(input.hasTimeRange)
  const limit = finiteNonNeg(input.limit) || 500
  const path_ok = list_path.includes('/audit')
  let tone: AuditSessionSummary['tone'] = 'unknown'
  if (!path_ok) tone = 'bad'
  else if (server_total != null && event_count < server_total) tone = 'warn'
  else if (path_ok) tone = 'ok'
  return {
    list_path,
    source,
    event_count,
    filtered_count,
    server_total,
    selected_user,
    action_filter,
    client_query,
    has_time_range,
    limit,
    preferred_list_path: preferred,
    path_ok,
    tone,
  }
}

function finiteNonNeg(v: unknown): number {
  if (!Number.isFinite(Number(v))) return 0
  return Math.max(0, Math.trunc(Number(v)))
}
