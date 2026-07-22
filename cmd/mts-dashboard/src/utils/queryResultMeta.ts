/** 查询响应元数据：path/count + admin_op（与 mts-server query*Response 对齐） */

export interface QueryResultMetaInput {
  path?: string
  row_count?: number
  series_count?: number
  database?: string
  measurement?: string
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
  rows?: unknown[]
  columns?: unknown[]
}

export function queryResultPath(
  server: QueryResultMetaInput | null | undefined,
  clientFallback: string,
): string {
  const p = String(server?.path || '').trim()
  return p || String(clientFallback || '').trim()
}

export function queryResultRowCount(
  server: QueryResultMetaInput | null | undefined,
  clientFallback: number,
): number {
  const n = Number(server?.row_count)
  if (Number.isFinite(n) && n >= 0) return Math.trunc(n)
  const fb = Number(clientFallback)
  return Number.isFinite(fb) && fb >= 0 ? Math.trunc(fb) : 0
}

export function queryResultSeriesCount(
  server: QueryResultMetaInput | null | undefined,
  clientFallback: number,
): number {
  const n = Number(server?.series_count)
  if (Number.isFinite(n) && n >= 0) return Math.trunc(n)
  const fb = Number(clientFallback)
  return Number.isFinite(fb) && fb >= 0 ? Math.trunc(fb) : 0
}

export function queryResultDatabase(
  server: QueryResultMetaInput | null | undefined,
  clientFallback = '',
): string {
  const d = String(server?.database || '').trim()
  return d || String(clientFallback || '').trim()
}

export function queryResultMeasurement(
  server: QueryResultMetaInput | null | undefined,
  clientFallback = '',
): string {
  const m = String(server?.measurement || '').trim()
  return m || String(clientFallback || '').trim()
}

/** 供 applyGlobalAdminOpStatus / parseAdminOpStatusPayload 使用 */
export function queryAdminOpPayload(server: QueryResultMetaInput | null | undefined): {
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
} | null {
  if (!server) return null
  if (
    server.admin_op_busy == null &&
    !server.op &&
    server.started_at_unix == null &&
    server.last == null
  ) {
    return null
  }
  return {
    admin_op_busy: server.admin_op_busy,
    op: server.op,
    started_at_unix: server.started_at_unix,
    last: server.last,
  }
}

/** 流式 end 帧 path/record_count（与 streamRecord type=end 对齐） */
export interface StreamEndMetaInput {
  path?: string
  format?: string
  record_count?: number
  database?: string
  measurement?: string
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
}

export function streamEndPath(
  server: StreamEndMetaInput | null | undefined,
  clientFallback = '/api/v1/data/query/stream',
): string {
  return queryResultPath(server, clientFallback)
}

export function streamEndRecordCount(
  server: StreamEndMetaInput | null | undefined,
  clientFallback: number,
): number {
  const n = Number(server?.record_count)
  if (Number.isFinite(n) && n >= 0) return Math.trunc(n)
  const fb = Number(clientFallback)
  return Number.isFinite(fb) && fb >= 0 ? Math.trunc(fb) : 0
}

export function streamAdminOpPayload(server: StreamEndMetaInput | null | undefined): {
  admin_op_busy?: boolean
  op?: string
  started_at_unix?: number
  last?: unknown
} | null {
  return queryAdminOpPayload(server)
}
