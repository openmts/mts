/** 实时授权导出（纯函数） */

import type { GrantRow } from './grantsSummary.ts'
import { escapeCSVCell } from './csvEscape.ts'

export function buildGrantsExport(
  rows: GrantRow[] | null | undefined,
  at = new Date(),
  meta?: {
    users_list_path?: string
    permissions_path_sample?: string
    user_count?: number
    database_count?: number
    partial_error_count?: number
  } | null,
): {
  kind: 'mts.access.grants'
  version: 2
  generated_at: string
  count: number
  users_list_path?: string
  permissions_path_sample?: string
  user_count?: number
  database_count?: number
  partial_error_count?: number
  grants: Array<{
    user: string
    role?: string
    disabled?: boolean
    database: string
    permission: string
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  const users_list_path = String(meta?.users_list_path || '').trim()
  const permissions_path_sample = String(meta?.permissions_path_sample || '').trim()
  const user_count =
    meta?.user_count != null && Number.isFinite(Number(meta.user_count))
      ? Math.max(0, Math.trunc(Number(meta.user_count)))
      : undefined
  const database_count =
    meta?.database_count != null && Number.isFinite(Number(meta.database_count))
      ? Math.max(0, Math.trunc(Number(meta.database_count)))
      : undefined
  const partial_error_count =
    meta?.partial_error_count != null && Number.isFinite(Number(meta.partial_error_count))
      ? Math.max(0, Math.trunc(Number(meta.partial_error_count)))
      : undefined
  return {
    kind: 'mts.access.grants',
    version: 2,
    generated_at: at.toISOString(),
    count: list.length,
    ...(users_list_path ? { users_list_path } : {}),
    ...(permissions_path_sample ? { permissions_path_sample } : {}),
    ...(user_count != null ? { user_count } : {}),
    ...(database_count != null ? { database_count } : {}),
    ...(partial_error_count != null ? { partial_error_count } : {}),
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
        .map(escapeCSVCell)
        .join(','),
    )
  }
  return lines.join('\n')
}
