/** 降采样策略详情展示字段（纯函数） */

export interface DownsamplePolicyDetailInput {
  name: string
  source_database?: string
  source_retention?: string
  source_measurement?: string
  target_database?: string
  target_retention?: string
  target_measurement?: string
  interval?: number
  refresh_interval?: number
  lookback?: number
  batch_size?: number
  enabled?: boolean
  group_by_tags?: string[]
  functions?: Array<{ function?: string; field?: string; as?: string }>
}

export interface DownsamplePolicyDetailField {
  key: string
  labelKey: string
  value: string
  mono?: boolean
}

export function formatDownsamplePolicyPath(p: DownsamplePolicyDetailInput): string {
  const srcRp = (p.source_retention || 'autogen').trim() || 'autogen'
  const tgtRp = (p.target_retention || 'autogen').trim() || 'autogen'
  const src = `${p.source_database || '?'}/${srcRp}/${p.source_measurement || '?'}`
  const tgt = `${p.target_database || '?'}/${tgtRp}/${p.target_measurement || '?'}`
  return `${src} → ${tgt}`
}

export function formatDownsampleFunctions(
  functions: DownsamplePolicyDetailInput['functions'] | null | undefined,
): string {
  const list = Array.isArray(functions) ? functions : []
  if (!list.length) return ''
  return list
    .map((f) => {
      const fn = (f.function || '').trim() || '?'
      const field = (f.field || '').trim() || '?'
      const as = (f.as || '').trim()
      return as ? `${fn}(${field}) as ${as}` : `${fn}(${field})`
    })
    .join(', ')
}

/** formatDuration: ns number → human; caller supplies formatter */
export function buildDownsamplePolicyDetailFields(
  p: DownsamplePolicyDetailInput | null | undefined,
  formatDuration: (ns: number | null | undefined) => string,
  empty = '—',
): DownsamplePolicyDetailField[] {
  if (!p || !p.name) return []
  const dur = (v: number | null | undefined) => {
    if (v == null || !Number.isFinite(Number(v)) || Number(v) <= 0) return empty
    try {
      return formatDuration(Number(v)) || empty
    } catch {
      return String(v)
    }
  }
  const tags = Array.isArray(p.group_by_tags) ? p.group_by_tags.filter(Boolean).join(', ') : ''
  const fns = formatDownsampleFunctions(p.functions)
  return [
    { key: 'name', labelKey: 'downsampleColName', value: p.name, mono: true },
    { key: 'path', labelKey: 'downsampleColPath', value: formatDownsamplePolicyPath(p), mono: true },
    {
      key: 'enabled',
      labelKey: 'downsampleStatusFilter',
      value: p.enabled ? 'enabled' : 'disabled',
    },
    { key: 'interval', labelKey: 'downsampleColInterval', value: dur(p.interval) },
    { key: 'refresh', labelKey: 'downsampleRefreshInterval', value: dur(p.refresh_interval) },
    { key: 'lookback', labelKey: 'downsampleLookback', value: dur(p.lookback) },
    {
      key: 'batch_size',
      labelKey: 'downsampleBatchSize',
      value: p.batch_size != null && p.batch_size > 0 ? String(p.batch_size) : empty,
    },
    { key: 'functions', labelKey: 'downsampleFunctions', value: fns || empty, mono: true },
    { key: 'group_by', labelKey: 'downsampleGroupByTags', value: tags || empty, mono: true },
  ]
}

/** 详情复制/导出用稳定 JSON（仅展示字段） */
export function buildDownsamplePolicyDetailJSON(
  p: DownsamplePolicyDetailInput | null | undefined,
): {
  kind: 'mts.downsample.policy_detail'
  version: 1
  policy: DownsamplePolicyDetailInput | null
} {
  if (!p || !p.name) {
    return { kind: 'mts.downsample.policy_detail', version: 1, policy: null }
  }
  return {
    kind: 'mts.downsample.policy_detail',
    version: 1,
    policy: {
      name: p.name,
      source_database: p.source_database,
      source_retention: p.source_retention,
      source_measurement: p.source_measurement,
      target_database: p.target_database,
      target_retention: p.target_retention,
      target_measurement: p.target_measurement,
      interval: p.interval,
      refresh_interval: p.refresh_interval,
      lookback: p.lookback,
      batch_size: p.batch_size,
      enabled: p.enabled,
      group_by_tags: p.group_by_tags,
      functions: p.functions,
    },
  }
}

export function downsamplePolicyDetailToJSONText(
  p: DownsamplePolicyDetailInput | null | undefined,
  space = 2,
): string {
  return JSON.stringify(buildDownsamplePolicyDetailJSON(p), null, space)
}

