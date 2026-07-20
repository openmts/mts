/** 侧栏导航分组（纯函数） */

export type NavSectionId = 'workspace' | 'access' | 'admin' | 'system'

export interface NavSectionDef {
  id: NavSectionId
  /** i18n MessageKey */
  labelKey: string
  paths: readonly string[]
}

/** 固定分组顺序与路径归属 */
export const NAV_SECTIONS: readonly NavSectionDef[] = [
  {
    id: 'workspace',
    labelKey: 'navSectionWorkspace',
    paths: ['/', '/query', '/write'],
  },
  {
    id: 'access',
    labelKey: 'navSectionAccess',
    paths: ['/users', '/access', '/access/grants'],
  },
  {
    id: 'admin',
    labelKey: 'navSectionAdmin',
    paths: [
      '/databases',
      '/observability/metrics',
      '/config',
      '/operations',
      '/downsample',
      '/audit',
      '/api-spec',
      '/storage',
      '/ops/readiness',
    ],
  },
  {
    id: 'system',
    labelKey: 'navSectionSystem',
    paths: ['/about', '/account'],
  },
]

export function sectionIdForPath(path: string): NavSectionId | null {
  for (const s of NAV_SECTIONS) {
    if (s.paths.includes(path)) return s.id
  }
  return null
}

export interface NavSectionGroup<T extends { to: string }> {
  id: NavSectionId
  labelKey: string
  items: T[]
}

/** 按固定分组顺序归类；空组丢弃；未知路径归入 system 末尾 */
export function groupNavItems<T extends { to: string }>(
  items: readonly T[],
): NavSectionGroup<T>[] {
  const buckets = new Map<NavSectionId, T[]>()
  for (const s of NAV_SECTIONS) buckets.set(s.id, [])
  const orphan: T[] = []

  for (const item of items) {
    const id = sectionIdForPath(item.to)
    if (id) buckets.get(id)!.push(item)
    else orphan.push(item)
  }

  const out: NavSectionGroup<T>[] = []
  for (const s of NAV_SECTIONS) {
    const list = buckets.get(s.id) || []
    if (!list.length) continue
    out.push({ id: s.id, labelKey: s.labelKey, items: list })
  }
  if (orphan.length) {
    const last = out.find((g) => g.id === 'system')
    if (last) last.items.push(...orphan)
    else out.push({ id: 'system', labelKey: 'navSectionSystem', items: orphan })
  }
  return out
}
