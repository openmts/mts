/** 配置页导出载荷（纯函数） */

export interface ConfigSchemaField {
  name: string
  description: string
}

export interface ConfigErrorCode {
  code: string
  http_status: number
  grpc_code: string
  description: string
}

export function buildEffectiveConfigExport(
  config: Record<string, unknown> | null | undefined,
  at = new Date(),
): {
  kind: 'mts.config.effective'
  version: 1
  generated_at: string
  config: Record<string, unknown>
} {
  return {
    kind: 'mts.config.effective',
    version: 1,
    generated_at: at.toISOString(),
    config: config && typeof config === 'object' ? config : {},
  }
}

export function buildConfigSchemaExport(
  fields: ConfigSchemaField[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.config.schema'
  version: 1
  generated_at: string
  count: number
  fields: ConfigSchemaField[]
} {
  const list = Array.isArray(fields) ? fields : []
  return {
    kind: 'mts.config.schema',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
    fields: list,
  }
}

export function buildErrorCodesExport(
  codes: ConfigErrorCode[] | null | undefined,
  at = new Date(),
): {
  kind: 'mts.error_codes'
  version: 1
  generated_at: string
  count: number
  codes: ConfigErrorCode[]
} {
  const list = Array.isArray(codes) ? codes : []
  return {
    kind: 'mts.error_codes',
    version: 1,
    generated_at: at.toISOString(),
    count: list.length,
    codes: list,
  }
}

export function formatConfigPretty(config: Record<string, unknown> | null | undefined): string {
  return JSON.stringify(config && typeof config === 'object' ? config : {}, null, 2)
}
