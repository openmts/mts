/** 运维页导出（纯函数） */

export function buildMaintenanceErrorsExport(
  errors: string[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.ops.maintenance_errors'
  version: 1
  generated_at: string
  count: number
  errors: string[]
} {
  const list = Array.isArray(errors) ? errors.map((e) => String(e ?? '')).filter(Boolean) : []
  return {
    kind: 'mts.ops.maintenance_errors',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
    errors: list,
  }
}

export function maintenanceErrorsToText(errors: string[] | null | undefined): string {
  const list = Array.isArray(errors) ? errors.map((e) => String(e ?? '')).filter(Boolean) : []
  return list.join('\n')
}
