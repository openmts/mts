/** 可商用就绪清单完成状态（localStorage 持久化，跨 Storage/Readiness 页共享） */

export const READINESS_STORAGE_KEY = 'mts.dashboard.readiness.v1'

/** 部署材料包本地提醒勾选（不计入就绪评分） */
export type DeployKitHintId = 'reviewed' | 'downloaded' | 'copied'

export interface ReadinessState {
  production: Record<string, boolean>
  edgeHttps: Record<string, boolean>
  backupSchedule: Record<string, boolean>
  /** 部署材料包本地提醒：查阅/下载/复制；人工签核仍为部署侧 */
  deployKit: Record<string, boolean>
  updatedAt?: string
}

export type ReadinessFlagSection = keyof Pick<
  ReadinessState,
  'production' | 'edgeHttps' | 'backupSchedule' | 'deployKit'
>

export function emptyReadinessState(): ReadinessState {
  return { production: {}, edgeHttps: {}, backupSchedule: {}, deployKit: {} }
}

export function loadReadinessState(
  storage: Pick<Storage, 'getItem'> | null = typeof localStorage !== 'undefined' ? localStorage : null,
  key = READINESS_STORAGE_KEY,
): ReadinessState {
  if (!storage) return emptyReadinessState()
  try {
    const raw = storage.getItem(key)
    if (!raw) return emptyReadinessState()
    const parsed = JSON.parse(raw) as Partial<ReadinessState>
    return {
      production: { ...(parsed.production ?? {}) },
      edgeHttps: { ...(parsed.edgeHttps ?? {}) },
      backupSchedule: { ...(parsed.backupSchedule ?? {}) },
      deployKit: { ...(parsed.deployKit ?? {}) },
      updatedAt: typeof parsed.updatedAt === 'string' ? parsed.updatedAt : undefined,
    }
  } catch {
    return emptyReadinessState()
  }
}

export function saveReadinessState(
  state: ReadinessState,
  storage: Pick<Storage, 'setItem'> | null = typeof localStorage !== 'undefined' ? localStorage : null,
  key = READINESS_STORAGE_KEY,
): ReadinessState {
  const next: ReadinessState = {
    production: { ...state.production },
    edgeHttps: { ...state.edgeHttps },
    backupSchedule: { ...state.backupSchedule },
    deployKit: { ...(state.deployKit ?? {}) },
    updatedAt: new Date().toISOString(),
  }
  if (!storage) return next
  try {
    storage.setItem(key, JSON.stringify(next))
  } catch {
    /* ignore quota / private mode */
  }
  return next
}

export function setReadinessFlag(
  section: ReadinessFlagSection,
  id: string,
  checked: boolean,
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof localStorage !== 'undefined' ? localStorage : null,
): ReadinessState {
  const cur = loadReadinessState(storage)
  const map = { ...cur[section], [id]: checked }
  if (!checked) delete map[id]
  return saveReadinessState({ ...cur, [section]: map }, storage)
}

export function completedIds(map: Record<string, boolean> | undefined): string[] {
  if (!map) return []
  return Object.entries(map)
    .filter(([, v]) => !!v)
    .map(([k]) => k)
}
