/** 查询页本地偏好 */

export interface QueryWorkbenchPrefs {
  showChart: boolean
  showRawFields: boolean
  showHistory: boolean
}

export const DEFAULT_QUERY_PREFS: QueryWorkbenchPrefs = {
  showChart: true,
  showRawFields: false,
  showHistory: false,
}

export function parseQueryPrefs(raw: unknown): QueryWorkbenchPrefs {
  if (!raw || typeof raw !== 'object') return { ...DEFAULT_QUERY_PREFS }
  const o = raw as Record<string, unknown>
  return {
    showChart: typeof o.showChart === 'boolean' ? o.showChart : DEFAULT_QUERY_PREFS.showChart,
    showRawFields: typeof o.showRawFields === 'boolean' ? o.showRawFields : DEFAULT_QUERY_PREFS.showRawFields,
    showHistory: typeof o.showHistory === 'boolean' ? o.showHistory : DEFAULT_QUERY_PREFS.showHistory,
  }
}

export function loadQueryPrefs(storage: Pick<Storage, 'getItem'> | null, key: string): QueryWorkbenchPrefs {
  if (!storage) return { ...DEFAULT_QUERY_PREFS }
  try {
    const raw = storage.getItem(key)
    if (!raw) return { ...DEFAULT_QUERY_PREFS }
    return parseQueryPrefs(JSON.parse(raw))
  } catch {
    return { ...DEFAULT_QUERY_PREFS }
  }
}

export function saveQueryPrefs(
  storage: Pick<Storage, 'setItem'> | null,
  key: string,
  prefs: QueryWorkbenchPrefs,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify(prefs))
  } catch {
    /* ignore */
  }
}
