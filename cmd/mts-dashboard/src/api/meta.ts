import { apiGet } from './client'

/** 服务端 list databases 历史字段名为 measurements，封装后对外统一为数据库名列表 */
interface MeasurementsPayload {
  measurements?: string[]
  databases?: string[]
}

export async function listDatabases(init: RequestInit = {}): Promise<string[]> {
  const data = await apiGet<MeasurementsPayload>('/api/v1/admin/databases', init)
  const names = data.databases ?? data.measurements ?? []
  return [...names].sort()
}

export async function listMeasurements(database: string, init: RequestInit = {}): Promise<string[]> {
  const data = await apiGet<MeasurementsPayload>(
    `/api/v1/data/databases/${encodeURIComponent(database)}/measurements`,
    init,
  )
  return [...(data.measurements ?? [])].sort()
}

export async function listRetentionPolicies(
  database: string,
  init: RequestInit = {},
): Promise<{ name: string; duration?: number }[]> {
  const data = await apiGet<{ policies: { name: string; duration?: number }[] }>(
    `/api/v1/admin/databases/${encodeURIComponent(database)}/retention-policies`,
    init,
  )
  return data.policies ?? []
}
