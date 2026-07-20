/** 浏览器在线状态（纯函数，便于单测） */

export type NetworkStatus = 'online' | 'offline'

export function networkStatusFromOnlineFlag(online: boolean | null | undefined): NetworkStatus {
  if (online === false) return 'offline'
  return 'online'
}

export function isOfflineStatus(status: NetworkStatus): boolean {
  return status === 'offline'
}

/** 读取 navigator.onLine；无 navigator 时视为 online（SSR/测试默认） */
export function readNavigatorOnline(nav: { onLine?: boolean } | null | undefined = typeof navigator !== 'undefined' ? navigator : null): boolean {
  if (!nav || typeof nav.onLine !== 'boolean') return true
  return nav.onLine
}
