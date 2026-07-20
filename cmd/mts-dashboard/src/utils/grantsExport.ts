/** 实时授权导出（纯函数） */

import type { GrantRow } from './grantsSummary.ts'

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export function buildGrantsExport(
  rows: GrantRow[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.access.grants'
  version: 1
  generated_at: string
  count: number
  grants: Array<{
    user: string
    role?: string
    disabled?: boolean
    database: string
    permission: string
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  return {
    kind: 'mts.access.grants',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
    grants: list.map((r) => ({
      user: r.user,
      role: r.role,
      disabled: r.disabled,
      database: r.database,
      permission: r.permission,
    })),
  }
}

export function grantsToCSV(rows: GrantRow[] | null | undefined): string {
  const header = ['user', 'role', 'disabled', 'database', 'permission']
  const lines = [header.join(',')]
  for (const r of rows || []) {
    lines.push(
      [r.user, r.role || '', r.disabled ? 'true' : 'false', r.database, r.permission]
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}
