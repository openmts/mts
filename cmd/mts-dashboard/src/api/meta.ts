import { apiGet, APIClientError } from './client'

/** 服务端 list databases 同时返回 databases（正式）与 measurements（兼容） */
interface MeasurementsPayload {
  measurements?: string[]
  databases?: string[]
}

export type MetaLoadSource = 'admin' | 'manual' | 'partial'

export interface ListDatabasesResult {
  names: string[]
  source: MetaLoadSource
  error?: string
}

export async function listDatabases(init: RequestInit = {}): Promise<string[]> {
  const result = await listDatabasesDetailed(init)
  if (result.error && !result.names.length) {
    throw new Error(result.error)
  }
  return result.names
}

/** 优先 admin 列表；403/权限失败时返回空列表并标记 manual，供页面手填降级 */
export async function listDatabasesDetailed(init: RequestInit = {}): Promise<ListDatabasesResult> {
  try {
    const data = await apiGet<MeasurementsPayload>('/api/v1/admin/databases', init)
    const names = data.databases ?? data.measurements ?? []
    return { names: [...names].sort(), source: 'admin' }
  } catch (e) {
    const msg = e instanceof Error ? e.message : '加载数据库列表失败'
    const denied = e instanceof APIClientError && (e.status === 403 || e.code === 'permission_denied')
    if (denied) {
      return { names: [], source: 'manual', error: '当前账号无库列表权限，请手动输入 database' }
    }
    return { names: [], source: 'manual', error: msg }
  }
}

export async function listMeasurements(database: string, init: RequestInit = {}): Promise<string[]> {
  if (!database.trim()) return []
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
  if (!database.trim()) return []
  try {
    const data = await apiGet<{ policies: { name: string; duration?: number }[] }>(
      `/api/v1/admin/databases/${encodeURIComponent(database)}/retention-policies`,
      init,
    )
    return data.policies ?? []
  } catch (e) {
    const denied = e instanceof APIClientError && (e.status === 403 || e.code === 'permission_denied')
    if (denied) return []
    throw e
  }
}
