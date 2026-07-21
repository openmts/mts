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

export function buildOpsStatsExport(
  input: {
    maintenance?: object | null
    compaction?: object | null
    memory?: object | null
    maintenance_errors?: string[] | null
  },
  at = new Date(),
): {
  kind: 'mts.ops.stats'
  version: 1
  generated_at: string
  maintenance: object | null
  compaction: object | null
  memory: object | null
  maintenance_errors: string[]
  maintenance_error_count: number
} {
  const errors = Array.isArray(input.maintenance_errors)
    ? input.maintenance_errors.map((e) => String(e ?? '')).filter(Boolean)
    : []
  return {
    kind: 'mts.ops.stats',
    version: 1,
    generated_at: at.toISOString(),
    maintenance: input.maintenance ?? null,
    compaction: input.compaction ?? null,
    memory: input.memory ?? null,
    maintenance_errors: errors,
    maintenance_error_count: errors.length,
  }
}

export function formatOpsStatsPretty(
  input: Parameters<typeof buildOpsStatsExport>[0],
  at = new Date(),
): string {
  return JSON.stringify(buildOpsStatsExport(input, at), null, 2)
}
