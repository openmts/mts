import type { QueryResultRow } from '@/composables/useQueryWorkbench'

export interface ChartPoint {
  x: number
  y: number
}

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

export function extractSeries(rows: QueryResultRow[], field: string): ChartPoint[] {
  const pts: ChartPoint[] = []
  for (const row of rows) {
    const raw = row.fields?.[field]
    let y: number | null = null
    if (typeof raw === 'number' && Number.isFinite(raw)) y = raw
    else if (raw && typeof raw === 'object') {
      const o = raw as Record<string, unknown>
      if (typeof o.float64 === 'number' && Number.isFinite(o.float64)) y = o.float64
      else if (typeof o.int64 === 'number' && Number.isFinite(o.int64)) y = o.int64
    }
    if (y === null) continue
    pts.push({ x: row.timestamp, y })
  }
  pts.sort((a, b) => a.x - b.x)
  return pts
}

export function buildPolyline(
  points: ChartPoint[],
  width: number,
  height: number,
  pad = 24,
): { path: string; minY: number; maxY: number; minX: number; maxX: number } {
  if (!points.length) return { path: '', minY: 0, maxY: 0, minX: 0, maxX: 0 }
  const xs = points.map((p) => p.x)
  const ys = points.map((p) => p.y)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  let minY = Math.min(...ys)
  let maxY = Math.max(...ys)
  if (minY === maxY) {
    minY -= 1
    maxY += 1
  }
  const w = Math.max(1, width - pad * 2)
  const h = Math.max(1, height - pad * 2)
  const scaleX = (x: number) => pad + ((x - minX) / Math.max(1, maxX - minX)) * w
  const scaleY = (y: number) => pad + (1 - (y - minY) / (maxY - minY)) * h
  const path = points
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${scaleX(p.x).toFixed(1)},${scaleY(p.y).toFixed(1)}`)
    .join(' ')
  return { path, minY, maxY, minX, maxX }
}
