/** 用户清单导出（纯函数） */

export interface UserExportRow {
  name: string
  display_name?: string
  role?: string
  disabled?: boolean
}

export function buildUsersExport(
  rows: UserExportRow[] | null | undefined,
  at = new Date(),
  meta?: { list_path?: string; batch_path?: string } | null,
): {
  kind: 'mts.users.inventory'
  version: 2
  generated_at: string
  count: number
  list_path?: string
  batch_path?: string
  users: Array<{
    name: string
    display_name?: string
    role?: string
    disabled: boolean
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  const list_path = String(meta?.list_path || '').trim()
  const batch_path = String(meta?.batch_path || '').trim()
  return {
    kind: 'mts.users.inventory',
    version: 2,
    generated_at: at.toISOString(),
    count: list.length,
    ...(list_path ? { list_path } : {}),
    ...(batch_path ? { batch_path } : {}),
    users: list.map((r) => ({
      name: r.name,
      display_name: r.display_name,
      role: r.role,
      disabled: Boolean(r.disabled),
    })),
  }
}

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export const USERS_CSV_HEADER = 'name,display_name,role,disabled'

export function userToCSVLine(r: UserExportRow): string {
  return [r.name, r.display_name || '', r.role || '', r.disabled ? 'true' : 'false']
    .map((c) => escapeCSV(String(c ?? '')))
    .join(',')
}

export function usersToCSV(rows: UserExportRow[] | null | undefined): string {
  const lines = [USERS_CSV_HEADER]
  for (const r of rows || []) lines.push(userToCSVLine(r))
  return lines.join('\n')
}
