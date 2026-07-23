/** Users 页列表 path / 批量结果对齐摘要（纯函数） */

export interface UsersMetaAlign {
  list_path: string
  preferred_list_path: string
  user_count: number
  filtered_count: number
  admin_count: number
  disabled_count: number
  selected_count: number
  batch_path: string
  path_ok: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export interface UsersBatchSummary {
  path: string
  ok_count: number
  skip_count: number
  fail_count: number
  cancelled: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

export const USERS_LIST_PATH = '/api/v1/users'
export const USERS_BATCH_DISABLED_PATH = '/api/v1/users/batch-disabled'

export function alignUsersMeta(input: {
  listPath?: string | null
  userCount?: number
  filteredCount?: number
  adminCount?: number
  disabledCount?: number
  selectedCount?: number
}): UsersMetaAlign {
  const list_path = String(input.listPath || '').trim() || USERS_LIST_PATH
  const user_count = finiteNonNeg(input.userCount)
  const filtered_count = finiteNonNeg(input.filteredCount)
  const admin_count = finiteNonNeg(input.adminCount)
  const disabled_count = finiteNonNeg(input.disabledCount)
  const selected_count = finiteNonNeg(input.selectedCount)
  const path_ok = list_path === USERS_LIST_PATH || list_path.startsWith('/api/v1/users')
  let tone: UsersMetaAlign['tone'] = 'unknown'
  if (!path_ok) tone = 'bad'
  else if (disabled_count > 0 && disabled_count === user_count && user_count > 0) tone = 'warn'
  else if (path_ok) tone = 'ok'
  return {
    list_path,
    preferred_list_path: USERS_LIST_PATH,
    user_count,
    filtered_count,
    admin_count,
    disabled_count,
    selected_count,
    batch_path: USERS_BATCH_DISABLED_PATH,
    path_ok,
    tone,
  }
}

export function buildUsersBatchSummary(input: {
  path?: string | null
  okCount?: number
  skipCount?: number
  failCount?: number
  cancelled?: boolean
}): UsersBatchSummary {
  const path = String(input.path || '').trim() || USERS_BATCH_DISABLED_PATH
  const ok_count = finiteNonNeg(input.okCount)
  const skip_count = finiteNonNeg(input.skipCount)
  const fail_count = finiteNonNeg(input.failCount)
  const cancelled = Boolean(input.cancelled)
  let tone: UsersBatchSummary['tone'] = 'ok'
  if (fail_count > 0) tone = 'bad'
  else if (cancelled || skip_count > 0) tone = 'warn'
  else if (ok_count === 0 && skip_count === 0 && fail_count === 0) tone = 'unknown'
  return { path, ok_count, skip_count, fail_count, cancelled, tone }
}

function finiteNonNeg(v: unknown): number {
  if (!Number.isFinite(Number(v))) return 0
  return Math.max(0, Math.trunc(Number(v)))
}
