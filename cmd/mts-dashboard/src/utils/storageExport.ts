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
