/** 数据面限额展示/校验纯函数 */

export interface DataLimitsView {
  maxWritePoints: number
  defaultQueryLimit: number
  maxQueryLimit: number
  path: string
}

export function normalizeDataLimits(raw: {
  max_write_points?: number
  default_query_limit?: number
  max_query_limit?: number
  path?: string
} | null | undefined): DataLimitsView {
  const n = (v: unknown) => {
    const x = Number(v)
    return Number.isFinite(x) && x >= 0 ? Math.trunc(x) : 0
  }
  return {
    maxWritePoints: n(raw?.max_write_points),
    defaultQueryLimit: n(raw?.default_query_limit),
    maxQueryLimit: n(raw?.max_query_limit),
    path: String(raw?.path || '/api/v1/data/limits').trim() || '/api/v1/data/limits',
  }
}

/** 查询 limit 是否超过服务端硬上限 */
export function queryLimitExceedsMax(limit: number, maxQueryLimit: number): boolean {
  if (!Number.isFinite(limit) || limit <= 0) return false
  if (!maxQueryLimit || maxQueryLimit <= 0) return false
  return limit > maxQueryLimit
}

/** 写入点数是否超过服务端硬上限 */
export function writePointsExceedsMax(points: number, maxWritePoints: number): boolean {
  if (!Number.isFinite(points) || points <= 0) return false
  if (!maxWritePoints || maxWritePoints <= 0) return false
  return points > maxWritePoints
}

export function clampQueryLimitInput(limit: number, maxQueryLimit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) return 0
  if (maxQueryLimit > 0 && limit > maxQueryLimit) return maxQueryLimit
  return Math.trunc(limit)
}
