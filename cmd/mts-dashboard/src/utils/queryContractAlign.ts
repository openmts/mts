/** Query 页与 data contract / limits 对齐摘要（纯函数） */

import type { DataContractView } from './dataContractView.ts'
import type { DataLimitsView } from './dataLimitsView.ts'

export type QueryModePath =
  | 'rows'
  | 'columns'
  | 'explain'
  | 'stream-row'
  | 'stream-column'
  | 'stats'

export interface QueryContractAlign {
  contract_path: string
  limits_path: string
  active_query_path: string
  max_query_limit: number | null
  default_query_limit: number | null
  limits_match_contract: boolean | null
  query_features_ok: boolean
  missing_query_features: string[]
  recommend_columns: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

const QUERY_FEATURE_IDS = [
  'query_result_meta',
  'query_stats_path',
  'query_stream_end_meta',
  'delete_response_meta',
  'data_limits',
  'meta_list_path',
] as const

export function activeQueryApiPath(mode: string): string {
  switch (mode) {
    case 'columns':
      return '/api/v1/data/query/columns'
    case 'explain':
      return '/api/v1/data/query/explain'
    case 'stream-row':
    case 'stream-column':
      return '/api/v1/data/query/stream'
    case 'stats':
      return '/api/v1/data/query/stats'
    case 'rows':
    default:
      return '/api/v1/data/query/rows'
  }
}

export function alignQueryContract(input: {
  contract?: DataContractView | null
  limits?: DataLimitsView | null
  queryMode?: string
}): QueryContractAlign {
  const contract = input.contract
  const limits = input.limits
  const mode = input.queryMode || 'rows'
  const active = activeQueryApiPath(mode)
  const missing: string[] = []
  if (contract?.loaded) {
    const enabled = new Set(contract.features.filter((f) => f.enabled).map((f) => f.id))
    for (const id of QUERY_FEATURE_IDS) {
      if (!enabled.has(id)) missing.push(id)
    }
  }
  let maxQ: number | null = null
  let defQ: number | null = null
  if (limits) {
    if (Number.isFinite(limits.maxQueryLimit) && limits.maxQueryLimit > 0) maxQ = limits.maxQueryLimit
    if (Number.isFinite(limits.defaultQueryLimit) && limits.defaultQueryLimit > 0) {
      defQ = limits.defaultQueryLimit
    }
  }
  if (maxQ == null && contract?.maxQueryLimit != null && contract.maxQueryLimit > 0) {
    maxQ = contract.maxQueryLimit
  }
  if (defQ == null && contract?.defaultQueryLimit != null && contract.defaultQueryLimit > 0) {
    defQ = contract.defaultQueryLimit
  }
  let limitsMatch: boolean | null = null
  if (
    contract?.loaded
    && limits
    && contract.maxQueryLimit != null
    && Number.isFinite(limits.maxQueryLimit)
  ) {
    limitsMatch = contract.maxQueryLimit === limits.maxQueryLimit
  }
  const queryFeaturesOk = contract?.loaded ? missing.length === 0 : false
  let tone: QueryContractAlign['tone'] = 'unknown'
  if (contract?.loaded) {
    if (!queryFeaturesOk) tone = 'bad'
    else if (limitsMatch === false) tone = 'warn'
    else tone = 'ok'
  } else if (limits) {
    tone = 'ok'
  }
  return {
    contract_path: contract?.path || '/api/v1/data/contract',
    limits_path: limits?.path || '/api/v1/data/limits',
    active_query_path: active,
    max_query_limit: maxQ,
    default_query_limit: defQ,
    limits_match_contract: limitsMatch,
    query_features_ok: queryFeaturesOk,
    missing_query_features: missing,
    recommend_columns: mode === 'rows',
    tone,
  }
}
