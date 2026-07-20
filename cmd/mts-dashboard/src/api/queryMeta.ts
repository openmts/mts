import { apiGet } from './client'
import type { QueryStatsData } from './types'

/** 引擎最近一次查询 stats 快照（GET /api/v1/data/query/stats） */
export async function fetchEngineQueryStats(init: RequestInit = {}): Promise<QueryStatsData> {
  const data = await apiGet<{ stats?: QueryStatsData }>('/api/v1/data/query/stats', init)
  return data.stats ?? {}
}
