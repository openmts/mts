/** 数据库清单导出（纯函数） */

export interface DatabaseExportRow {
  name: string
  measurement_count?: number
  retention_policy_count?: number
  loaded?: boolean
}

export function buildDatabasesExport(
  rows: DatabaseExportRow[] | null | undefined,
  at = new Date(),
  meta?: { list_path?: string; source?: string } | null,
): {
  kind: 'mts.databases.inventory'
  version: 2
  generated_at: string
  count: number
  list_path?: string
  source?: string
  databases: Array<{
    name: string
    measurement_count?: number
    retention_policy_count?: number
    loaded: boolean
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  const list_path = String(meta?.list_path || '').trim()
  const source = String(meta?.source || '').trim()
  return {
    kind: 'mts.databases.inventory',
    version: 2,
    generated_at: at.toISOString(),
    count: list.length,
    ...(list_path ? { list_path } : {}),
    ...(source ? { source } : {}),
    databases: list.map((r) => ({
      name: r.name,
      measurement_count: r.measurement_count,
      retention_policy_count: r.retention_policy_count,
      loaded: Boolean(r.loaded),
    })),
  }
}

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export function databasesToCSV(rows: DatabaseExportRow[] | null | undefined): string {
  const header = ['name', 'measurement_count', 'retention_policy_count', 'loaded']
  const lines = [header.join(',')]
  for (const r of rows || []) {
    lines.push(
      [
        r.name,
        r.measurement_count ?? '',
        r.retention_policy_count ?? '',
        r.loaded ? 'true' : 'false',
      ]
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}
