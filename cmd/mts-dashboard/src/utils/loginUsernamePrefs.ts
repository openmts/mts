/** 登录用户名记忆（纯函数 + storage；不存密码） */

export const LOGIN_USERNAME_PREFS_KEY = 'mts.dashboard.login.username.v1'

export function normalizeLoginUsername(raw: unknown): string {
  return String(raw ?? '').trim().slice(0, 128)
}

export function loadLoginUsernamePref(
  storage: Pick<Storage, 'getItem'> | null,
  key = LOGIN_USERNAME_PREFS_KEY,
): string {
  if (!storage) return ''
  try {
    return normalizeLoginUsername(storage.getItem(key))
  } catch {
    return ''
  }
}

export function saveLoginUsernamePref(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  value: string,
  key = LOGIN_USERNAME_PREFS_KEY,
): void {
  if (!storage) return
  try {
    const s = normalizeLoginUsername(value)
    if (!s) storage.removeItem(key)
    else storage.setItem(key, s)
  } catch {
    /* ignore */
  }
}

export function clearLoginUsernamePref(
  storage: Pick<Storage, 'removeItem'> | null,
  key = LOGIN_USERNAME_PREFS_KEY,
): void {
  if (!storage) return
  try {
    storage.removeItem(key)
  } catch {
    /* ignore */
  }
}
