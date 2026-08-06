/** 审计事件导出（纯函数） */

import { escapeCSVCell } from './csvEscape.ts'

export interface AuditExportEvent {
  time: string
  user_name: string
  action: string
  database?: string
  detail?: string
}

export interface AuditExportMeta {
  list_path?: string
  source?: string
  server_total?: number | null
  filtered_count?: number
  selected_user?: string
  action_filter?: string
  client_query?: string
  limit?: number
}

export const AUDIT_CSV_HEADER = 'time,user_name,action,database,detail'

export function auditEventToCSVLine(e: AuditExportEvent): string {
  return [e.time, e.user_name, e.action, e.database || '', e.detail || '']
    .map(escapeCSVCell)
    .join(',')
}

export function auditEventsToCSV(events: AuditExportEvent[]): string {
  const lines = [AUDIT_CSV_HEADER]
  for (const e of events || []) lines.push(auditEventToCSVLine(e))
  return lines.join('\n')
}

export function buildAuditExport(
  events: AuditExportEvent[] | null | undefined,
  at = new Date(),
  meta?: AuditExportMeta | null,
): {
  kind: 'mts.audit.export'
  version: 2
  generated_at: string
  count: number
  list_path?: string
  source?: string
  server_total?: number | null
  filtered_count?: number
  selected_user?: string
  action_filter?: string
  client_query?: string
  limit?: number
  events: AuditExportEvent[]
} {
  const list = Array.isArray(events) ? events : []
  const list_path = String(meta?.list_path || '').trim()
  const source = String(meta?.source || '').trim()
  const selected_user = String(meta?.selected_user || '').trim()
  const action_filter = String(meta?.action_filter || '').trim()
  const client_query = String(meta?.client_query || '').trim()
  const filtered_count =
    meta?.filtered_count != null && Number.isFinite(Number(meta.filtered_count))
      ? Math.max(0, Math.trunc(Number(meta.filtered_count)))
      : undefined
  const limit =
    meta?.limit != null && Number.isFinite(Number(meta.limit))
      ? Math.max(0, Math.trunc(Number(meta.limit)))
      : undefined
  const server_total =
    meta?.server_total == null
      ? undefined
      : Number.isFinite(Number(meta.server_total))
        ? Math.max(0, Math.trunc(Number(meta.server_total)))
        : null
  return {
    kind: 'mts.audit.export',
    version: 2,
    generated_at: at.toISOString(),
    count: list.length,
    ...(list_path ? { list_path } : {}),
    ...(source ? { source } : {}),
    ...(server_total !== undefined ? { server_total } : {}),
    ...(filtered_count != null ? { filtered_count } : {}),
    ...(selected_user ? { selected_user } : {}),
    ...(action_filter ? { action_filter } : {}),
    ...(client_query ? { client_query } : {}),
    ...(limit != null ? { limit } : {}),
    events: list.map((e) => ({
      time: e.time,
      user_name: e.user_name,
      action: e.action,
      database: e.database,
      detail: e.detail,
    })),
  }
}
