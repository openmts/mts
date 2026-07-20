/** 降采样策略清单导出（纯函数） */

export interface DownsampleExportRow {
  name: string
  source_database?: string
  source_measurement?: string
  target_database?: string
  target_measurement?: string
  interval?: number | string
  enabled?: boolean
}

export function buildDownsampleExport(
  rows: DownsampleExportRow[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.downsample.policies'
  version: 1
  generated_at: string
  count: number
  policies: Array<{
    name: string
    source_database?: string
    source_measurement?: string
    target_database?: string
    target_measurement?: string
    interval?: number | string
    enabled: boolean
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  return {
    kind: 'mts.downsample.policies',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
    policies: list.map((r) => ({
      name: r.name,
      source_database: r.source_database,
      source_measurement: r.source_measurement,
      target_database: r.target_database,
      target_measurement: r.target_measurement,
      interval: r.interval,
      enabled: Boolean(r.enabled),
    })),
  }
}

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

export function downsampleToCSV(rows: DownsampleExportRow[] | null | undefined): string {
  const header = [
    'name',
    'source_database',
    'source_measurement',
    'target_database',
    'target_measurement',
    'interval',
    'enabled',
  ]
  const lines = [header.join(',')]
  for (const r of rows || []) {
    lines.push(
      [
        r.name,
        r.source_database || '',
        r.source_measurement || '',
        r.target_database || '',
        r.target_measurement || '',
        r.interval ?? '',
        r.enabled ? 'true' : 'false',
      ]
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}
