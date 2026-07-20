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
): {
  kind: 'mts.users.inventory'
  version: 1
  generated_at: string
  count: number
  users: Array<{
    name: string
    display_name?: string
    role?: string
    disabled: boolean
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  return {
    kind: 'mts.users.inventory',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
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

export function usersToCSV(rows: UserExportRow[] | null | undefined): string {
  const header = ['name', 'display_name', 'role', 'disabled']
  const lines = [header.join(',')]
  for (const r of rows || []) {
    lines.push(
      [r.name, r.display_name || '', r.role || '', r.disabled ? 'true' : 'false']
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}
