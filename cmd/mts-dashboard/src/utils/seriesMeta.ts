/** Series/Fields 元数据展示与表单填充（纯函数） */

export type SeriesLike = {
  id?: number
  measurement?: string
  tags?: Record<string, string>
}

/** 稳定排序后的 tags 表达式：k=v,k2=v2 */
export function tagsToExpr(tags?: Record<string, string> | null): string {
  if (!tags) return ''
  return Object.keys(tags)
    .sort()
    .map((k) => `${k}=${tags[k] ?? ''}`)
    .join(',')
}

/** 列表行标题：优先 tags 表达式，否则 #id / measurement */
export function seriesLabel(s: SeriesLike, empty = '(no tags)'): string {
  const expr = tagsToExpr(s.tags)
  if (expr) return expr
  if (s.id != null) return `#${s.id}`
  if (s.measurement) return s.measurement
  return empty
}

/** 截断 series 列表，避免 UI 膨胀 */
export function capSeriesList<T>(items: T[], max: number): { items: T[]; truncated: boolean; total: number } {
  const total = items.length
  if (max <= 0) return { items: [], truncated: total > 0, total }
  if (total <= max) return { items: [...items], truncated: false, total }
  return { items: items.slice(0, max), truncated: true, total }
}

/** 字段名列表（去空） */
export function fieldNames(fields: { name?: string }[]): string[] {
  const out: string[] = []
  for (const f of fields) {
    const n = String(f?.name || '').trim()
    if (n) out.push(n)
  }
  return out
}

/** 将选中字段名追加到 fields 文本（逗号分隔，去重） */
export function appendFieldToken(current: string, name: string): string {
  const token = name.trim()
  if (!token) return current
  const parts = current
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  if (parts.includes(token)) return parts.join(',')
  parts.push(token)
  return parts.join(',')
}


/** 按标签表达式或自由文本筛选 series（客户端） */
export function filterSeriesList<T extends SeriesLike>(items: T[], query: string): T[] {
  const q = query.trim().toLowerCase()
  if (!q) return [...items]
  return items.filter((s) => {
    const label = seriesLabel(s).toLowerCase()
    if (label.includes(q)) return true
    if (s.measurement && s.measurement.toLowerCase().includes(q)) return true
    if (s.id != null && String(s.id).includes(q)) return true
    if (s.tags) {
      for (const [k, v] of Object.entries(s.tags)) {
        if (k.toLowerCase().includes(q) || String(v).toLowerCase().includes(q)) return true
        if (`${k}=${v}`.toLowerCase().includes(q)) return true
      }
    }
    return false
  })
}
