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
  retryable?: boolean
  category?: string
  remediation?: string
  dashboard_path?: string
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

export interface EffectiveConfigSummary {
  path: string
  top_level_keys: number
  sections: string[]
  leaf_count: number
  nested_object_count: number
  sensitive_key_hits: number
  sample_keys: string[]
}

const SENSITIVE_KEY_RE = /(password|secret|token|credential|private_key|api_key)/i

function countLeaves(value: unknown, depth = 0): { leaves: number; objects: number; sensitive: number } {
  if (value === null || typeof value !== 'object') {
    return { leaves: 1, objects: 0, sensitive: 0 }
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return { leaves: 1, objects: 0, sensitive: 0 }
    let leaves = 0
    let objects = 0
    let sensitive = 0
    for (const item of value) {
      const c = countLeaves(item, depth + 1)
      leaves += c.leaves
      objects += c.objects
      sensitive += c.sensitive
    }
    return { leaves, objects, sensitive }
  }
  const obj = value as Record<string, unknown>
  const keys = Object.keys(obj)
  if (keys.length === 0) return { leaves: 1, objects: 1, sensitive: 0 }
  let leaves = 0
  let objects = 1
  let sensitive = 0
  for (const k of keys) {
    if (SENSITIVE_KEY_RE.test(k)) sensitive += 1
    const c = countLeaves(obj[k], depth + 1)
    leaves += c.leaves
    objects += c.objects
    sensitive += c.sensitive
  }
  return { leaves, objects, sensitive }
}

/** 有效配置扫视摘要：顶层分区、叶子数、敏感键命中（仅键名，不含值） */
export function summarizeEffectiveConfig(
  config: Record<string, unknown> | null | undefined,
  path = '/api/v1/admin/config/effective',
): EffectiveConfigSummary | null {
  if (!config || typeof config !== 'object' || Array.isArray(config)) return null
  const sections = Object.keys(config).sort()
  const counts = countLeaves(config)
  return {
    path,
    top_level_keys: sections.length,
    sections,
    leaf_count: counts.leaves,
    nested_object_count: counts.objects,
    sensitive_key_hits: counts.sensitive,
    sample_keys: sections.slice(0, 8),
  }
}
