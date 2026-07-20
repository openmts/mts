/** 登录 TTL 输入记忆（秒数字符串；空表示服务端默认） */

export const LOGIN_TTL_PREFS_KEY = 'mts.dashboard.login.ttl.v1'

export function loadLoginTTLPref(
  storage: Pick<Storage, 'getItem'> | null,
  key = LOGIN_TTL_PREFS_KEY,
): string {
  if (!storage) return ''
  try {
    return storage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

export function saveLoginTTLPref(
  storage: Pick<Storage, 'setItem' | 'removeItem'> | null,
  value: string,
  key = LOGIN_TTL_PREFS_KEY,
): void {
  if (!storage) return
  try {
    const s = String(value ?? '').trim()
    if (!s) storage.removeItem(key)
    else storage.setItem(key, s)
  } catch {
    /* ignore */
  }
}
