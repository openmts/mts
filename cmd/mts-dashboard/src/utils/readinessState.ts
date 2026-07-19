/** 可商用就绪清单完成状态（localStorage 持久化，跨 Storage/Readiness 页共享） */

export const READINESS_STORAGE_KEY = 'mts.dashboard.readiness.v1'

/** 部署材料包本地提醒勾选（不计入就绪评分） */
export type DeployKitHintId = 'reviewed' | 'downloaded' | 'copied'

/** 部署侧人工签核证据备注（自由文本；不计入评分、不代表验收完成） */
export interface SignoffNotes {
  /** 边缘证书/HSTS 人工验收证据摘要 */
  edgeHttps?: string
  /** 异地/跨主机备份执行证据摘要 */
  backupOffsite?: string
  /** 备份失败告警通道证据摘要 */
  backupAlert?: string
}

export interface ReadinessState {
  production: Record<string, boolean>
  edgeHttps: Record<string, boolean>
  backupSchedule: Record<string, boolean>
  /** 部署材料包本地提醒：查阅/下载/复制；人工签核仍为部署侧 */
  deployKit: Record<string, boolean>
  /** 部署侧签核证据备注（可选；导出归档用） */
  signoffNotes?: SignoffNotes
  updatedAt?: string
}

export type ReadinessFlagSection = keyof Pick<
  ReadinessState,
  'production' | 'edgeHttps' | 'backupSchedule' | 'deployKit'
>

export function emptyReadinessState(): ReadinessState {
  return { production: {}, edgeHttps: {}, backupSchedule: {}, deployKit: {}, signoffNotes: {} }
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
      signoffNotes: normalizeSignoffNotes(parsed.signoffNotes),
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
    signoffNotes: normalizeSignoffNotes(state.signoffNotes),
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


const SIGNOFF_KEYS = ['edgeHttps', 'backupOffsite', 'backupAlert'] as const

export function normalizeSignoffNotes(raw: unknown): SignoffNotes {
  if (raw == null || typeof raw !== 'object') return {}
  const o = raw as Record<string, unknown>
  const out: SignoffNotes = {}
  for (const k of SIGNOFF_KEYS) {
    const v = o[k]
    if (typeof v === 'string') {
      const trimmed = v.trim()
      if (trimmed) out[k] = trimmed.slice(0, 2000)
    }
  }
  return out
}

export function setSignoffNote(
  field: keyof SignoffNotes,
  value: string,
  storage: Pick<Storage, 'getItem' | 'setItem'> | null = typeof localStorage !== 'undefined' ? localStorage : null,
): ReadinessState {
  const cur = loadReadinessState(storage)
  const notes = normalizeSignoffNotes(cur.signoffNotes)
  const trimmed = value.trim().slice(0, 2000)
  if (trimmed) notes[field] = trimmed
  else delete notes[field]
  return saveReadinessState({ ...cur, signoffNotes: notes }, storage)
}

export function completedIds(map: Record<string, boolean> | undefined): string[] {
  if (!map) return []
  return Object.entries(map)
    .filter(([, v]) => !!v)
    .map(([k]) => k)
}
