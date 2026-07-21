/** 跨标签页从 storage 重载会话快照（纯函数） */

export const AUTH_STORAGE_KEYS = {
  token: 'mts_bearer_token',
  user: 'mts_user_name',
  role: 'mts_user_role',
  expiresAt: 'mts_token_expires_at',
  mustChange: 'mts_must_change_password',
} as const

export interface AuthStorageSnapshot {
  token: string
  user: string
  role: string
  expiresAt: string
  mustChange: boolean
}

export function readAuthStorageSnapshot(
  get: (key: string) => string | null | undefined,
): AuthStorageSnapshot {
  const must = get(AUTH_STORAGE_KEYS.mustChange)
  return {
    token: String(get(AUTH_STORAGE_KEYS.token) ?? ''),
    user: String(get(AUTH_STORAGE_KEYS.user) ?? ''),
    role: String(get(AUTH_STORAGE_KEYS.role) ?? ''),
    expiresAt: String(get(AUTH_STORAGE_KEYS.expiresAt) ?? ''),
    mustChange: must === '1' || must === 'true',
  }
}

/** 判断 storage 变更 key 是否会影响会话内存态 */
export function isAuthStorageKey(key: string | null | undefined): boolean {
  if (!key) return false
  return (
    key === AUTH_STORAGE_KEYS.token ||
    key === AUTH_STORAGE_KEYS.user ||
    key === AUTH_STORAGE_KEYS.role ||
    key === AUTH_STORAGE_KEYS.expiresAt ||
    key === AUTH_STORAGE_KEYS.mustChange
  )
}
