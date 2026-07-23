/** Access Grants 页 path / 覆盖率对齐摘要（纯函数） */

export const USERS_LIST_PATH = '/api/v1/users'
export const USER_PERMISSIONS_PATH_TEMPLATE = '/api/v1/users/{name}/database-permissions'

export interface AccessGrantsMetaAlign {
  users_list_path: string
  preferred_users_list_path: string
  permissions_path_sample: string
  grant_count: number
  filtered_count: number
  user_count: number
  database_count: number
  partial_error_count: number
  selected_count: number
  path_ok: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export function preferredPermissionsPath(userName?: string | null): string {
  const name = String(userName || '').trim()
  if (!name) return USER_PERMISSIONS_PATH_TEMPLATE
  return `/api/v1/users/${encodeURIComponent(name)}/database-permissions`
}

export function alignAccessGrantsMeta(input: {
  usersListPath?: string | null
  permissionsPathSample?: string | null
  grantCount?: number
  filteredCount?: number
  userCount?: number
  databaseCount?: number
  partialErrorCount?: number
  selectedCount?: number
}): AccessGrantsMetaAlign {
  const users_list_path = String(input.usersListPath || '').trim() || USERS_LIST_PATH
  const permissions_path_sample =
    String(input.permissionsPathSample || '').trim() || USER_PERMISSIONS_PATH_TEMPLATE
  const grant_count = finiteNonNeg(input.grantCount)
  const filtered_count = finiteNonNeg(input.filteredCount)
  const user_count = finiteNonNeg(input.userCount)
  const database_count = finiteNonNeg(input.databaseCount)
  const partial_error_count = finiteNonNeg(input.partialErrorCount)
  const selected_count = finiteNonNeg(input.selectedCount)
  const path_ok =
    users_list_path.includes('/api/v1/users') &&
    permissions_path_sample.includes('database-permissions')
  let tone: AccessGrantsMetaAlign['tone'] = 'unknown'
  if (!path_ok) tone = 'bad'
  else if (partial_error_count > 0) tone = 'warn'
  else if (path_ok) tone = 'ok'
  return {
    users_list_path,
    preferred_users_list_path: USERS_LIST_PATH,
    permissions_path_sample,
    grant_count,
    filtered_count,
    user_count,
    database_count,
    partial_error_count,
    selected_count,
    path_ok,
    tone,
  }
}

function finiteNonNeg(v: unknown): number {
  if (!Number.isFinite(Number(v))) return 0
  return Math.max(0, Math.trunc(Number(v)))
}
