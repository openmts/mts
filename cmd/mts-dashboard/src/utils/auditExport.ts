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

export function auditEventsToCSV(events: AuditExportEvent[]): string {
  const header = ['time', 'user_name', 'action', 'database', 'detail']
  const lines = [header.join(',')]
  for (const e of events || []) {
    const cols = [e.time, e.user_name, e.action, e.database || '', e.detail || ''].map((v) =>
      escapeCSV(String(v ?? '')),
    )
    lines.push(cols.join(','))
  }
  return lines.join('\n')
}
