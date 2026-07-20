/** 侧栏布局偏好（纯函数 + storage 读写） */

export const SIDEBAR_PREFS_KEY = 'mts.dashboard.sidebar.prefs.v1'

export interface SidebarPrefs {
  /** 桌面端窄轨（仅图标） */
  collapsed: boolean
}

export const DEFAULT_SIDEBAR_PREFS: SidebarPrefs = {
  collapsed: false,
}

export function parseSidebarPrefs(raw: unknown): SidebarPrefs {
  if (!raw || typeof raw !== 'object') return { ...DEFAULT_SIDEBAR_PREFS }
  const o = raw as Record<string, unknown>
  return {
    collapsed: typeof o.collapsed === 'boolean' ? o.collapsed : DEFAULT_SIDEBAR_PREFS.collapsed,
  }
}

export function loadSidebarPrefs(
  storage: Pick<Storage, 'getItem'> | null,
  key = SIDEBAR_PREFS_KEY,
): SidebarPrefs {
  if (!storage) return { ...DEFAULT_SIDEBAR_PREFS }
  try {
    const raw = storage.getItem(key)
    if (!raw) return { ...DEFAULT_SIDEBAR_PREFS }
    return parseSidebarPrefs(JSON.parse(raw))
  } catch {
    return { ...DEFAULT_SIDEBAR_PREFS }
  }
}

export function saveSidebarPrefs(
  storage: Pick<Storage, 'setItem'> | null,
  prefs: SidebarPrefs,
  key = SIDEBAR_PREFS_KEY,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify(prefs))
  } catch {
    /* ignore */
  }
}
