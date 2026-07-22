/** 存储配置导出包装（纯函数） */

export function buildStorageConfigExport(
  exportData: object | null | undefined,
  at = new Date(),
): {
  kind: 'mts.storage.export'
  version: 1
  generated_at: string
  export: object
} {
  const payload = exportData && typeof exportData === 'object' ? exportData : {}
  return {
    kind: 'mts.storage.export',
    version: 1,
    generated_at: at.toISOString(),
    export: payload,
  }
}

export function formatStorageExportPretty(exportData: object | null | undefined): string {
  return JSON.stringify(buildStorageConfigExport(exportData), null, 2)
}


export interface StorageExportSummary {
  path: string
  generated_at: string
  user_count: number
  grant_user_count: number
  grant_total: number
  healthy: boolean | null
  ready: boolean | null
  reason_count: number
  config_keys: number
}

export function summarizeStorageExport(
  exportData: {
    generated_at?: string
    config?: Record<string, unknown>
    health?: Record<string, unknown>
    users?: unknown[]
    grants?: Record<string, unknown[]>
  } | null | undefined,
  path = '/api/v1/admin/storage/export',
): StorageExportSummary | null {
  if (!exportData || typeof exportData !== 'object') return null
  const users = Array.isArray(exportData.users) ? exportData.users : []
  const grants = exportData.grants && typeof exportData.grants === 'object' ? exportData.grants : {}
  let grantTotal = 0
  for (const v of Object.values(grants)) {
    if (Array.isArray(v)) grantTotal += v.length
  }
  const health = exportData.health && typeof exportData.health === 'object' ? exportData.health : {}
  const healthy = typeof health.healthy === 'boolean' ? health.healthy : null
  const ready = typeof health.ready === 'boolean' ? health.ready : null
  const reasons = Array.isArray(health.reasons) ? health.reasons : []
  const config = exportData.config && typeof exportData.config === 'object' ? exportData.config : {}
  return {
    path,
    generated_at: String(exportData.generated_at || ''),
    user_count: users.length,
    grant_user_count: Object.keys(grants).length,
    grant_total: grantTotal,
    healthy,
    ready,
    reason_count: reasons.length,
    config_keys: Object.keys(config).length,
  }
}
