/** series 列表本地筛选（id / measurement / tag key-value） */

export type SeriesFilterable = {
  id?: number | string
  measurement?: string
  tags?: Record<string, string>
}

export function seriesMatchesLocal(series: SeriesFilterable, query: string): boolean {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  if (String(series.id ?? '').toLowerCase().includes(needle)) return true
  if ((series.measurement || '').toLowerCase().includes(needle)) return true
  for (const [key, value] of Object.entries(series.tags || {})) {
    if (key.toLowerCase().includes(needle) || String(value).toLowerCase().includes(needle)) {
      return true
    }
  }
  return false
}

export function filterSeriesListLocal<T extends SeriesFilterable>(series: T[], query: string): T[] {
  const needle = query.trim()
  if (!needle) return series
  return series.filter((item) => seriesMatchesLocal(item, needle))
}
