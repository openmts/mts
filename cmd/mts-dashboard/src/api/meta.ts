import { apiGet, APIClientError } from './client'
import { formatCaughtError } from '@/utils/apiError'

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
    const denied = e instanceof APIClientError && (e.status === 403 || e.code === 'permission_denied')
    if (denied) {
      return {
        names: [],
        source: 'manual',
        error: formatCaughtError({ code: 'permission_denied', status: 403 }),
      }
    }
    return { names: [], source: 'manual', error: formatCaughtError(e) }
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

export type RetentionPolicyMeta = { name: string; duration?: number }

export interface ListRetentionPoliciesResult {
  policies: RetentionPolicyMeta[]
  /** data 面优先；admin 回退；均失败则为 manual */
  source: 'data' | 'admin' | 'manual'
  error?: string
}

/**
 * 列出 RP：优先 data 面只读路径（有 database read 即可），
 * 再回退 admin 路径；均失败返回空并标记 manual。
 */
export async function listRetentionPoliciesDetailed(
  database: string,
  init: RequestInit = {},
): Promise<ListRetentionPoliciesResult> {
  if (!database.trim()) return { policies: [], source: 'manual' }
  const dataPath = `/api/v1/data/databases/${encodeURIComponent(database)}/retention-policies`
  const adminPath = `/api/v1/admin/databases/${encodeURIComponent(database)}/retention-policies`
  try {
    const data = await apiGet<{ policies?: RetentionPolicyMeta[] }>(dataPath, init)
    return { policies: data.policies ?? [], source: 'data' }
  } catch (e) {
    const denied = e instanceof APIClientError && (e.status === 403 || e.code === 'permission_denied')
    // 继续尝试 admin（admin token 且 data 路径异常时）
    try {
      const data = await apiGet<{ policies?: RetentionPolicyMeta[] }>(adminPath, init)
      return { policies: data.policies ?? [], source: 'admin' }
    } catch (e2) {
      const denied2 = e2 instanceof APIClientError && (e2.status === 403 || e2.code === 'permission_denied')
      if (denied || denied2) {
        return {
          policies: [],
          source: 'manual',
          error: formatCaughtError(denied2 ? e2 : e),
        }
      }
      return { policies: [], source: 'manual', error: formatCaughtError(e2) }
    }
  }
}

export async function listRetentionPolicies(
  database: string,
  init: RequestInit = {},
): Promise<RetentionPolicyMeta[]> {
  const result = await listRetentionPoliciesDetailed(database, init)
  return result.policies
}

export type FieldMeta = { name: string; type?: number }

export async function listFields(
  database: string,
  measurement: string,
  init: RequestInit = {},
): Promise<FieldMeta[]> {
  if (!database.trim() || !measurement.trim()) return []
  const data = await apiGet<{ fields?: FieldMeta[] }>(
    `/api/v1/data/databases/${encodeURIComponent(database)}/measurements/${encodeURIComponent(measurement)}/fields`,
    init,
  )
  return [...(data.fields ?? [])].sort((a, b) => a.name.localeCompare(b.name))
}

export type SeriesMeta = {
  id?: number
  measurement?: string
  tags?: Record<string, string>
}

export async function listSeries(
  database: string,
  measurement: string,
  tags?: Record<string, string>,
  init: RequestInit = {},
): Promise<SeriesMeta[]> {
  if (!database.trim() || !measurement.trim()) return []
  const qs = new URLSearchParams()
  if (tags) {
    for (const [k, v] of Object.entries(tags)) {
      if (k) qs.set(k, v)
    }
  }
  const q = qs.toString()
  const path =
    `/api/v1/data/databases/${encodeURIComponent(database)}/measurements/${encodeURIComponent(measurement)}/series` +
    (q ? `?${q}` : '')
  const data = await apiGet<{ series?: SeriesMeta[] }>(path, init)
  return data.series ?? []
}
