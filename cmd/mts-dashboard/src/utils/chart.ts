import type { QueryResultRow } from '@/api/types'

export interface ChartPoint {
  x: number
  y: number
}

export interface ChartSeries {
  key: string
  label: string
  color: string
  points: ChartPoint[]
}

const PALETTE = [
  '#2563eb', '#dc2626', '#059669', '#d97706', '#7c3aed',
  '#db2777', '#0891b2', '#4f46e5', '#65a30d', '#ea580c',
]

export function extractNumericFieldNames(rows: QueryResultRow[]): string[] {
  const names = new Set<string>()
  for (const row of rows) {
    const fields = row.fields || {}
    for (const [k, v] of Object.entries(fields)) {
      if (typeof v === 'number' && Number.isFinite(v)) names.add(k)
      else if (v && typeof v === 'object') {
        const o = v as Record<string, unknown>
        if (typeof o.float64 === 'number' || typeof o.int64 === 'number') names.add(k)
      }
    }
  }
  return [...names].sort()
}

export function numericFieldValue(raw: unknown): number | null {
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  if (raw && typeof raw === 'object') {
    const o = raw as Record<string, unknown>
    if (typeof o.float64 === 'number' && Number.isFinite(o.float64)) return o.float64
    if (typeof o.int64 === 'number' && Number.isFinite(o.int64)) return o.int64
  }
  return null
}

export function seriesKeyFromTags(tags: Record<string, string> | null | undefined): string {
  if (!tags || !Object.keys(tags).length) return ''
  return Object.keys(tags)
    .sort()
    .map((k) => `${k}=${tags[k]}`)
    .join(',')
}

/** 兼容旧单 series API */
export function extractSeries(rows: QueryResultRow[], field: string): ChartPoint[] {
  const series = extractMultiSeries(rows, field, 1)
  return series[0]?.points ?? []
}

/** 按 tag 组合拆分多 series；maxSeries 限制图例数量 */
export function extractMultiSeries(
  rows: QueryResultRow[],
  field: string,
  maxSeries = 8,
): ChartSeries[] {
  const buckets = new Map<string, ChartPoint[]>()
  for (const row of rows) {
    const y = numericFieldValue(row.fields?.[field])
    if (y === null) continue
    const key = seriesKeyFromTags(row.tags)
    const list = buckets.get(key) ?? []
    list.push({ x: row.timestamp, y })
    buckets.set(key, list)
  }
  const entries = [...buckets.entries()]
    .map(([key, points]) => {
      points.sort((a, b) => a.x - b.x)
      return { key, points }
    })
    .sort((a, b) => b.points.length - a.points.length || a.key.localeCompare(b.key))
    .slice(0, Math.max(1, maxSeries))

  return entries.map((e, i) => ({
    key: e.key || '__default__',
    label: e.key || 'series',
    color: PALETTE[i % PALETTE.length],
    points: e.points,
  }))
}

export function polyline(points: ChartPoint[], width: number, height: number, pad = 24): string {
  if (!points.length) return ''
  const xs = points.map((p) => p.x)
  const ys = points.map((p) => p.y)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)
  const dx = maxX - minX || 1
  const dy = maxY - minY || 1
  const innerW = width - pad * 2
  const innerH = height - pad * 2
  return points
    .map((p, i) => {
      const x = pad + ((p.x - minX) / dx) * innerW
      const y = pad + (1 - (p.y - minY) / dy) * innerH
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

export function boundsOfSeries(series: ChartSeries[]): { minX: number; maxX: number; minY: number; maxY: number } | null {
  const pts = series.flatMap((s) => s.points)
  if (!pts.length) return null
  return {
    minX: Math.min(...pts.map((p) => p.x)),
    maxX: Math.max(...pts.map((p) => p.x)),
    minY: Math.min(...pts.map((p) => p.y)),
    maxY: Math.max(...pts.map((p) => p.y)),
  }
}

export function polylineInBounds(
  points: ChartPoint[],
  bounds: { minX: number; maxX: number; minY: number; maxY: number },
  width: number,
  height: number,
  pad = 24,
): string {
  if (!points.length) return ''
  const dx = bounds.maxX - bounds.minX || 1
  const dy = bounds.maxY - bounds.minY || 1
  const innerW = width - pad * 2
  const innerH = height - pad * 2
  return points
    .map((p, i) => {
      const x = pad + ((p.x - bounds.minX) / dx) * innerW
      const y = pad + (1 - (p.y - bounds.minY) / dy) * innerH
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}
