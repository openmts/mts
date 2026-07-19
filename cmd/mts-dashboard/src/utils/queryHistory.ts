/** 查询历史排序与展示标题（纯函数，便于单测） */

export interface QueryHistoryForm {
  database: string
  retention_policy: string
  measurement: string
  start_time: string
  end_time: string
  fields: string
  tags?: string
  order?: 'asc' | 'desc' | ''
  offset?: string
  limit: string
  aggregates?: string
  window?: string
  group_tags?: string
}

export interface QueryHistoryRecord {
  id: string
  at: number
  mode: string
  form: QueryHistoryForm
  name?: string
  pinned?: boolean
}

/** pinned 优先，同组内按 at 降序 */
export function sortHistoryItems<T extends { at: number; pinned?: boolean }>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const ap = a.pinned ? 1 : 0
    const bp = b.pinned ? 1 : 0
    if (ap !== bp) return bp - ap
    return b.at - a.at
  })
}

/** 展示标题：自定义名优先，否则 database/measurement · mode */
export function historyItemTitle(item: {
  name?: string
  mode: string
  form: { database?: string; measurement?: string }
}): string {
  const custom = item.name?.trim()
  if (custom) return custom
  const db = item.form.database?.trim() || '?'
  const meas = item.form.measurement?.trim() || '*'
  return `${db}/${meas} · ${item.mode}`
}

/**
 * 在容量上限内合并新条目：优先保留 pinned，再按时间保留较新项。
 * 新项始终保留在结果中（若自身也超限则由调用方保证 id 唯一）。
 */
export function mergeHistoryCap<T extends { id: string; at: number; pinned?: boolean }>(
  items: T[],
  max: number,
): T[] {
  if (max <= 0) return []
  if (items.length <= max) return sortHistoryItems(items)
  const sorted = sortHistoryItems(items)
  const pinned = sorted.filter((x) => x.pinned)
  if (pinned.length >= max) return pinned.slice(0, max)
  const rest = sorted.filter((x) => !x.pinned)
  return [...pinned, ...rest].slice(0, max)
}

/** 从 localStorage 原始 JSON 规范化条目 */
export function normalizeHistoryItems(raw: unknown): QueryHistoryRecord[] {
  if (!Array.isArray(raw)) return []
  const out: QueryHistoryRecord[] = []
  for (const x of raw) {
    if (!x || typeof x !== 'object') continue
    const o = x as Record<string, unknown>
    if (typeof o.id !== 'string' || typeof o.at !== 'number' || typeof o.mode !== 'string') continue
    if (!o.form || typeof o.form !== 'object') continue
    const form = o.form as QueryHistoryForm
    out.push({
      id: o.id,
      at: o.at,
      mode: o.mode,
      form,
      name: typeof o.name === 'string' ? o.name : undefined,
      pinned: Boolean(o.pinned),
    })
  }
  return out
}
