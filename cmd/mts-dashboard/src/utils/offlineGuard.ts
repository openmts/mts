/** 离线时禁止变更类写操作（纯函数） */

export type OfflineGuardKind = 'write' | 'ops' | 'delete' | 'admin'

/** 浏览器离线时阻断会改变服务端状态的操作 */
export function isOfflineWriteBlocked(offline: boolean | null | undefined): boolean {
  return offline === true
}

/**
 * 若应阻断则返回 true。
 * online 未知（null/undefined）不阻断，避免 SSR/测试误伤。
 */
export function shouldBlockOfflineMutation(offline: boolean | null | undefined): boolean {
  return isOfflineWriteBlocked(offline)
}
