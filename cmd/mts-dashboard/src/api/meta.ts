import { apiGet, APIClientError } from './client'
import { formatCaughtError } from '@/utils/apiError'

/** 服务端 list databases 同时返回 databases（正式）与 measurements（兼容） */
interface MeasurementsPayload {
  measurements?: string[]
  databases?: string[]
}

export type MetaLoadSource = 'admin' | 'data' | 'manual' | 'partial'

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

/**
 * 列出 database：
 * 1) 优先 data 面只读路径（有任一可读库即可，非 admin 友好）
 * 2) 回退 admin 路径
 * 3) 均失败则 manual 手填
 */
export async function listDatabasesDetailed(init: RequestInit = {}): Promise<ListDatabasesResult> {
  const dataPath = '/api/v1/data/databases'
  const adminPath = '/api/v1/admin/databases'
  try {
    const data = await apiGet<MeasurementsPayload>(dataPath, init)
    const names = data.databases ?? data.measurements ?? []
    return { names: [...names].sort(), source: 'data' as MetaLoadSource }
  } catch (e) {
    try {
      const data = await apiGet<MeasurementsPayload>(adminPath, init)
      const names = data.databases ?? data.measurements ?? []
      return { names: [...names].sort(), source: 'admin' }
    } catch (e2) {
      const denied =
        (e instanceof APIClientError && (e.status === 403 || e.code === 'permission_denied')) ||
        (e2 instanceof APIClientError && (e2.status === 403 || e2.code === 'permission_denied'))
      if (denied) {
        return {
          names: [],
          source: 'manual',
          error: formatCaughtError(e2 instanceof APIClientError ? e2 : e),
        }
      }
      return { names: [], source: 'manual', error: formatCaughtError(e2) }
    }
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

export type ListSeriesResult = {
  series: SeriesMeta[]
  total: number
  truncated: boolean
  limit: number
  offset: number
}

export type ListSeriesOptions = {
  tags?: Record<string, string>
  limit?: number
  offset?: number
  q?: string
  init?: RequestInit
}

/** 列出 series：支持 tag 过滤与 limit（服务端截断并返回 total/truncated） */
export async function listSeriesDetailed(
  database: string,
  measurement: string,
  opts: ListSeriesOptions = {},
): Promise<ListSeriesResult> {
  if (!database.trim() || !measurement.trim()) {
    return { series: [], total: 0, truncated: false, limit: opts.limit ?? 0, offset: opts.offset ?? 0 }
  }
  const qs = new URLSearchParams()
  if (opts.tags) {
    for (const [k, v] of Object.entries(opts.tags)) {
      if (k) qs.set(k, v)
    }
  }
  if (opts.limit != null && opts.limit > 0) qs.set('limit', String(opts.limit))
  if (opts.offset != null && opts.offset > 0) qs.set('offset', String(opts.offset))
  if (opts.q && opts.q.trim()) qs.set('q', opts.q.trim())
  const q = qs.toString()
  const path =
    `/api/v1/data/databases/${encodeURIComponent(database)}/measurements/${encodeURIComponent(measurement)}/series` +
    (q ? `?${q}` : '')
  const data = await apiGet<{
    series?: SeriesMeta[]
    total?: number
    truncated?: boolean
    limit?: number
    offset?: number
  }>(path, opts.init)
  const series = data.series ?? []
  const total = typeof data.total === 'number' ? data.total : series.length
  return {
    series,
    total,
    truncated: !!data.truncated,
    limit: typeof data.limit === 'number' ? data.limit : opts.limit ?? 0,
    offset: typeof data.offset === 'number' ? data.offset : opts.offset ?? 0,
  }
}

export async function listSeries(
  database: string,
  measurement: string,
  tags?: Record<string, string>,
  init: RequestInit = {},
): Promise<SeriesMeta[]> {
  const result = await listSeriesDetailed(database, measurement, { tags, init })
  return result.series
}
