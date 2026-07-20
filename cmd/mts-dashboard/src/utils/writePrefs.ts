/** 写入页本地偏好（默认偏向 TypedBatch 列式写入） */

export type WriteModePref = 'form' | 'line' | 'prometheus' | 'typed'

export interface WriteWorkbenchPrefs {
  writeMode: WriteModePref
  usePointsTyped: boolean
  syncWrite: boolean
}

export const WRITE_PREFS_KEY = 'mts.dashboard.write.prefs.v1'

export const DEFAULT_WRITE_PREFS: WriteWorkbenchPrefs = {
  writeMode: 'typed',
  usePointsTyped: true,
  syncWrite: false,
}

const MODES = new Set<WriteModePref>(['form', 'line', 'prometheus', 'typed'])

export function parseWritePrefs(raw: unknown): WriteWorkbenchPrefs {
  if (!raw || typeof raw !== 'object') return { ...DEFAULT_WRITE_PREFS }
  const o = raw as Record<string, unknown>
  const mode = typeof o.writeMode === 'string' && MODES.has(o.writeMode as WriteModePref)
    ? (o.writeMode as WriteModePref)
    : DEFAULT_WRITE_PREFS.writeMode
  return {
    writeMode: mode,
    usePointsTyped:
      typeof o.usePointsTyped === 'boolean' ? o.usePointsTyped : DEFAULT_WRITE_PREFS.usePointsTyped,
    syncWrite: typeof o.syncWrite === 'boolean' ? o.syncWrite : DEFAULT_WRITE_PREFS.syncWrite,
  }
}

export function loadWritePrefs(
  storage: Pick<Storage, 'getItem'> | null,
  key = WRITE_PREFS_KEY,
): WriteWorkbenchPrefs {
  if (!storage) return { ...DEFAULT_WRITE_PREFS }
  try {
    const raw = storage.getItem(key)
    if (!raw) return { ...DEFAULT_WRITE_PREFS }
    return parseWritePrefs(JSON.parse(raw))
  } catch {
    return { ...DEFAULT_WRITE_PREFS }
  }
}

export function saveWritePrefs(
  storage: Pick<Storage, 'setItem'> | null,
  prefs: WriteWorkbenchPrefs,
  key = WRITE_PREFS_KEY,
): void {
  if (!storage) return
  try {
    storage.setItem(key, JSON.stringify(prefs))
  } catch {
    /* ignore */
  }
}
