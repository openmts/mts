/** 降采样策略清单导出（纯函数） */

export interface DownsampleExportRow {
  name: string
  source_database?: string
  source_measurement?: string
  source_retention?: string
  target_database?: string
  target_measurement?: string
  target_retention?: string
  interval?: number | string
  refresh_interval?: number | string
  lookback?: number | string
  batch_size?: number
  enabled?: boolean
}

export function buildDownsampleExport(
  rows: DownsampleExportRow[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.downsample.policies'
  version: 2
  generated_at: string
  count: number
  policies: Array<{
    name: string
    source_database?: string
    source_measurement?: string
    source_retention?: string
    target_database?: string
    target_measurement?: string
    target_retention?: string
    interval?: number | string
    refresh_interval?: number | string
    lookback?: number | string
    batch_size?: number
    enabled: boolean
  }>
} {
  const list = Array.isArray(rows) ? rows : []
  return {
    kind: 'mts.downsample.policies',
    version: 2,
    generated_at: at.toISOString(),
    count: list.length,
    policies: list.map((r) => ({
      name: r.name,
      source_database: r.source_database,
      source_measurement: r.source_measurement,
      source_retention: r.source_retention,
      target_database: r.target_database,
      target_measurement: r.target_measurement,
      target_retention: r.target_retention,
      interval: r.interval,
      refresh_interval: r.refresh_interval,
      lookback: r.lookback,
      batch_size: r.batch_size,
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
    'source_retention',
    'target_database',
    'target_measurement',
    'target_retention',
    'interval',
    'refresh_interval',
    'lookback',
    'batch_size',
    'enabled',
  ]
  const lines = [header.join(',')]
  for (const r of rows || []) {
    lines.push(
      [
        r.name,
        r.source_database || '',
        r.source_measurement || '',
        r.source_retention || '',
        r.target_database || '',
        r.target_measurement || '',
        r.target_retention || '',
        r.interval ?? '',
        r.refresh_interval ?? '',
        r.lookback ?? '',
        r.batch_size ?? '',
        r.enabled ? 'true' : 'false',
      ]
        .map((c) => escapeCSV(String(c ?? '')))
        .join(','),
    )
  }
  return lines.join('\n')
}
