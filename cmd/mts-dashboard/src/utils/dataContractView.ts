/** 数据面契约快照视图（与 GET /api/v1/data/contract 对齐） */

export interface DataContractFeatureInput {
  id?: string
  path?: string
  description?: string
  enabled?: boolean
}

export interface DataContractInput {
  version?: number
  path?: string
  max_write_points?: number
  default_query_limit?: number
  max_query_limit?: number
  features?: DataContractFeatureInput[] | null
  admin_op_busy?: boolean
  op?: string
}

export interface DataContractView {
  loaded: boolean
  path: string
  version: number
  maxWritePoints: number | null
  defaultQueryLimit: number | null
  maxQueryLimit: number | null
  features: Array<{ id: string; path: string; enabled: boolean; description: string }>
  enabledCount: number
  totalFeatures: number
  missingRequired: string[]
  complete: boolean
}

const REQUIRED_FEATURE_IDS = [
  'write_accepted_points',
  'write_response_mode',
  'write_response_retention',
  'query_result_meta',
  'query_stats_path',
  'query_stream_end_meta',
  'delete_response_meta',
  'data_limits',
  'meta_list_path',
] as const

export function requiredDataContractFeatureIds(): readonly string[] {
  return REQUIRED_FEATURE_IDS
}

function finiteOrNull(n: unknown): number | null {
  const v = Number(n)
  if (!Number.isFinite(v)) return null
  return Math.trunc(v)
}

export function buildDataContractView(
  input: DataContractInput | null | undefined,
): DataContractView {
  if (!input) {
    return {
      loaded: false,
      path: '/api/v1/data/contract',
      version: 0,
      maxWritePoints: null,
      defaultQueryLimit: null,
      maxQueryLimit: null,
      features: [],
      enabledCount: 0,
      totalFeatures: 0,
      missingRequired: [...REQUIRED_FEATURE_IDS],
      complete: false,
    }
  }
  const features = (Array.isArray(input.features) ? input.features : [])
    .map((f) => ({
      id: String(f?.id || '').trim(),
      path: String(f?.path || '').trim(),
      enabled: !!f?.enabled,
      description: String(f?.description || '').trim(),
    }))
    .filter((f) => f.id)
  const enabled = new Set(features.filter((f) => f.enabled).map((f) => f.id))
  const missingRequired = REQUIRED_FEATURE_IDS.filter((id) => !enabled.has(id))
  const path = String(input.path || '').trim() || '/api/v1/data/contract'
  const version = finiteOrNull(input.version) ?? 0
  return {
    loaded: true,
    path,
    version: version > 0 ? version : 0,
    maxWritePoints: finiteOrNull(input.max_write_points),
    defaultQueryLimit: finiteOrNull(input.default_query_limit),
    maxQueryLimit: finiteOrNull(input.max_query_limit),
    features,
    enabledCount: enabled.size,
    totalFeatures: features.length,
    missingRequired,
    complete: missingRequired.length === 0,
  }
}

export function formatDataContractHandoffLine(view: DataContractView): string {
  if (!view.loaded) return 'data_contract=unavailable'
  const limits = [
    `max_write=${view.maxWritePoints ?? '—'}`,
    `default_query=${view.defaultQueryLimit ?? '—'}`,
    `max_query=${view.maxQueryLimit ?? '—'}`,
  ].join(' ')
  const feat = `features=${view.enabledCount}/${Math.max(view.totalFeatures, REQUIRED_FEATURE_IDS.length)}`
  const miss = view.missingRequired.length
    ? ` missing=${view.missingRequired.join(',')}`
    : ' complete'
  return `data_contract v${view.version || 1} path=${view.path} ${limits} ${feat}${miss}`
}


/** 验收包顶层 data_contract 摘要（机器可读 + 一行文案） */
export interface AcceptanceDataContractSummary {
  loaded: boolean
  complete: boolean
  path: string
  version: number
  max_write_points: number | null
  default_query_limit: number | null
  max_query_limit: number | null
  enabled_count: number
  total_features: number
  missing_required: string[]
  summary_line: string
}

export function toAcceptanceDataContractSummary(
  input: DataContractInput | DataContractView | null | undefined,
): AcceptanceDataContractSummary {
  const view: DataContractView =
    input && typeof input === 'object' && 'loaded' in input && 'complete' in input && 'features' in input
      ? (input as DataContractView)
      : buildDataContractView(input as DataContractInput | null | undefined)
  return {
    loaded: view.loaded,
    complete: view.complete,
    path: view.path,
    version: view.version,
    max_write_points: view.maxWritePoints,
    default_query_limit: view.defaultQueryLimit,
    max_query_limit: view.maxQueryLimit,
    enabled_count: view.enabledCount,
    total_features: view.totalFeatures,
    missing_required: [...view.missingRequired],
    summary_line: formatDataContractHandoffLine(view),
  }
}

export function assertAcceptanceDataContractShape(
  value: unknown,
): value is AcceptanceDataContractSummary {
  if (!value || typeof value !== 'object') return false
  const v = value as Record<string, unknown>
  if (typeof v.loaded !== 'boolean' || typeof v.complete !== 'boolean') return false
  if (typeof v.path !== 'string' || typeof v.summary_line !== 'string') return false
  if (!Array.isArray(v.missing_required)) return false
  return true
}
