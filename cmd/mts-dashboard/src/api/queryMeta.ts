import { apiGet } from './client'
import type { QueryStatsData } from './types'

export interface EngineQueryStatsResult {
  stats: QueryStatsData
  adminOp?: {
    admin_op_busy?: boolean
    op?: string
    started_at_unix?: number
    last?: unknown
  }
}

/** 引擎最近一次查询 stats 快照（GET /api/v1/data/query/stats，含 admin_op_busy/last） */
export async function fetchEngineQueryStats(init: RequestInit = {}): Promise<EngineQueryStatsResult> {
  const data = await apiGet<{
    stats?: QueryStatsData
    admin_op_busy?: boolean
    op?: string
    started_at_unix?: number
    last?: unknown
  }>('/api/v1/data/query/stats', init)
  return {
    stats: data.stats ?? {},
    adminOp: {
      admin_op_busy: data.admin_op_busy,
      op: data.op,
      started_at_unix: data.started_at_unix,
      last: data.last,
    },
  }
}
