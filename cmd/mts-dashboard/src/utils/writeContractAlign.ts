/** Write 页与 data contract / limits 对齐摘要（纯函数） */

import type { DataContractView } from './dataContractView.ts'
import type { DataLimitsView } from './dataLimitsView.ts'

export interface WriteContractAlign {
  contract_path: string
  limits_path: string
  preferred_write_path: string
  max_write_points: number | null
  limits_match_contract: boolean | null
  write_features_ok: boolean
  missing_write_features: string[]
  recommend_typed: boolean
  tone: 'ok' | 'warn' | 'bad' | 'unknown'
}

const WRITE_FEATURE_IDS = [
  'write_accepted_points',
  'write_response_mode',
  'write_response_retention',
  'data_limits',
] as const

export function preferredWriteApiPath(mode: string, usePointsTyped: boolean): string {
  if (mode === 'typed') return '/api/v1/data/write/typed'
  if (usePointsTyped) return '/api/v1/data/write/points-typed'
  return '/api/v1/data/write'
}

export function alignWriteContract(input: {
  contract?: DataContractView | null
  limits?: DataLimitsView | null
  writeMode?: string
  usePointsTyped?: boolean
}): WriteContractAlign {
  const contract = input.contract
  const limits = input.limits
  const mode = input.writeMode || 'typed'
  const usePointsTyped = !!input.usePointsTyped
  const preferred = preferredWriteApiPath(mode, usePointsTyped)
  const missing: string[] = []
  if (contract?.loaded) {
    const enabled = new Set(contract.features.filter((f) => f.enabled).map((f) => f.id))
    for (const id of WRITE_FEATURE_IDS) {
      if (!enabled.has(id)) missing.push(id)
    }
  }
  let maxWrite: number | null = null
  if (limits && Number.isFinite(limits.maxWritePoints) && limits.maxWritePoints > 0) {
    maxWrite = limits.maxWritePoints
  } else if (contract?.maxWritePoints != null && contract.maxWritePoints > 0) {
    maxWrite = contract.maxWritePoints
  }
  let limitsMatch: boolean | null = null
  if (
    contract?.loaded
    && limits
    && contract.maxWritePoints != null
    && Number.isFinite(limits.maxWritePoints)
  ) {
    limitsMatch = contract.maxWritePoints === limits.maxWritePoints
  }
  const writeFeaturesOk = contract?.loaded ? missing.length === 0 : false
  let tone: WriteContractAlign['tone'] = 'unknown'
  if (contract?.loaded) {
    if (!writeFeaturesOk) tone = 'bad'
    else if (limitsMatch === false) tone = 'warn'
    else tone = mode === 'typed' ? 'ok' : 'warn'
  } else if (limits) {
    tone = mode === 'typed' ? 'ok' : 'warn'
  }
  return {
    contract_path: contract?.path || '/api/v1/data/contract',
    limits_path: limits?.path || '/api/v1/data/limits',
    preferred_write_path: preferred,
    max_write_points: maxWrite,
    limits_match_contract: limitsMatch,
    write_features_ok: writeFeaturesOk,
    missing_write_features: missing,
    recommend_typed: mode !== 'typed',
    tone,
  }
}
