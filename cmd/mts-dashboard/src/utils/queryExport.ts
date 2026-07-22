/** 查询结果 JSON 导出（含服务端 meta，与 writeExport 对齐） */

export interface QueryResultExportInput {
  mode?: string
  path?: string
  database?: string
  measurement?: string
  row_count?: number | null
  series_count?: number | null
  query?: Record<string, unknown> | null
  rows?: unknown[] | null
  columns?: unknown[] | null
  raw?: string | null
  stats?: unknown
  preview_only?: boolean
}

export function buildQueryResultExport(
  input: QueryResultExportInput | null | undefined,
  at = new Date(),
): {
  kind: 'mts.query.result'
  version: 1
  generated_at: string
  mode: string
  path: string
  database: string
  measurement: string
  row_count: number | null
  series_count: number | null
  preview_only: boolean
  query: Record<string, unknown> | null
  rows: unknown[] | null
  columns: unknown[] | null
  raw: string | null
  stats: unknown
} {
  const src = input || {}
  const n = (v: unknown): number | null => {
    const x = Number(v)
    return Number.isFinite(x) && x >= 0 ? Math.trunc(x) : null
  }
  return {
    kind: 'mts.query.result',
    version: 1,
    generated_at: at.toISOString(),
    mode: String(src.mode || ''),
    path: String(src.path || ''),
    database: String(src.database || ''),
    measurement: String(src.measurement || ''),
    row_count: n(src.row_count),
    series_count: n(src.series_count),
    preview_only: Boolean(src.preview_only),
    query: src.query && typeof src.query === 'object' ? { ...src.query } : null,
    rows: Array.isArray(src.rows) ? src.rows : null,
    columns: Array.isArray(src.columns) ? src.columns : null,
    raw: typeof src.raw === 'string' ? src.raw : null,
    stats: src.stats ?? null,
  }
}
