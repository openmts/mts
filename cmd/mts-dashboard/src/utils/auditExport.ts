/** 审计事件导出（纯函数） */

export interface AuditExportEvent {
  time: string
  user_name: string
  action: string
  database?: string
  detail?: string
}

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export const AUDIT_CSV_HEADER = 'time,user_name,action,database,detail'

export function auditEventToCSVLine(e: AuditExportEvent): string {
  return [e.time, e.user_name, e.action, e.database || '', e.detail || '']
    .map((v) => escapeCSV(String(v ?? '')))
    .join(',')
}

export function auditEventsToCSV(events: AuditExportEvent[]): string {
  const lines = [AUDIT_CSV_HEADER]
  for (const e of events || []) lines.push(auditEventToCSVLine(e))
  return lines.join('\n')
}
