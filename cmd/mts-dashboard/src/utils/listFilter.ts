/** 列表筛选纯函数 */

export function normalizeFilterQuery(q: string): string {
  return String(q || '').trim().toLowerCase()
}

export function filterByTextFields<T>(
  items: T[],
  query: string,
  fields: (item: T) => Array<string | undefined | null>,
): T[] {
  const q = normalizeFilterQuery(query)
  if (!q) return items
  return items.filter((item) =>
    fields(item).some((f) => String(f ?? '').toLowerCase().includes(q)),
  )
}

export type UsersStatusFilter = '' | 'active' | 'disabled'

export function filterUsers<
  T extends { name: string; display_name?: string; role?: string; disabled?: boolean },
>(users: T[], query: string, role: string, status: UsersStatusFilter = ''): T[] {
  let list = filterByTextFields(users, query, (u) => [u.name, u.display_name, u.role])
  const r = normalizeFilterQuery(role)
  if (r) list = list.filter((u) => normalizeFilterQuery(u.role || '') === r)
  if (status === 'active') list = list.filter((u) => !u.disabled)
  if (status === 'disabled') list = list.filter((u) => !!u.disabled)
  return list
}

export function filterByName<T extends { name: string }>(items: T[], query: string): T[] {
  return filterByTextFields(items, query, (x) => [x.name])
}

export type DownsampleEnabledFilter = '' | 'enabled' | 'disabled'

export function filterDownsamplePolicies<
  T extends {
    name: string
    source_database?: string
    source_measurement?: string
    target_database?: string
    target_measurement?: string
    enabled?: boolean
  },
>(items: T[], query: string, enabled: DownsampleEnabledFilter = ''): T[] {
  let list = filterByTextFields(items, query, (p) => [
    p.name,
    p.source_database,
    p.source_measurement,
    p.target_database,
    p.target_measurement,
  ])
  if (enabled === 'enabled') list = list.filter((p) => !!p.enabled)
  if (enabled === 'disabled') list = list.filter((p) => !p.enabled)
  return list
}


export function filterAccessMatrixRows<
  T extends {
    areaKey?: string
    area?: { zh?: string; en?: string } | string
    capability?: { zh?: string; en?: string } | string
    route?: string
    notes?: { zh?: string; en?: string } | string
  },
>(
  items: T[],
  query: string,
  pickText: (v: unknown) => string,
): T[] {
  const q = normalizeFilterQuery(query)
  if (!q) return items
  return items.filter((r) => {
    const fields = [
      pickText(r.area),
      pickText(r.capability),
      r.route || '',
      pickText(r.notes),
      r.areaKey || '',
    ]
    return fields.some((f) => String(f).toLowerCase().includes(q))
  })
}
