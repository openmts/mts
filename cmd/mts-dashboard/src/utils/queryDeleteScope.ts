/** 范围删除确认摘要（纯函数） */

export type QueryDeleteScopeInput = {
  database?: unknown
  retention_policy?: unknown
  measurement?: unknown
  tags?: unknown
  start_time?: unknown
  end_time?: unknown
}

export type QueryDeleteScope = {
  database: string
  retention_policy: string
  measurement: string
  tagsExpr: string
  start_time: string
  end_time: string
  hasTimeBound: boolean
}

function str(v: unknown, fallback = ''): string {
  if (v == null) return fallback
  const s = String(v).trim()
  return s || fallback
}

export function tagsToShortExpr(tags: unknown): string {
  if (!tags || typeof tags !== 'object' || Array.isArray(tags)) return ''
  const o = tags as Record<string, unknown>
  return Object.keys(o)
    .sort()
    .map((k) => `${k}=${o[k] == null ? '' : String(o[k])}`)
    .join(',')
}

export function buildQueryDeleteScope(query: QueryDeleteScopeInput): QueryDeleteScope {
  const start = str(query.start_time)
  const end = str(query.end_time)
  return {
    database: str(query.database, '*'),
    retention_policy: str(query.retention_policy, 'autogen'),
    measurement: str(query.measurement, '*'),
    tagsExpr: tagsToShortExpr(query.tags),
    start_time: start || '—',
    end_time: end || '—',
    hasTimeBound: Boolean(start || end),
  }
}

/** 多行纯文本摘要，供确认对话框 message 使用 */
export function formatQueryDeleteScopeMessage(
  scope: QueryDeleteScope,
  labels: {
    database: string
    retention: string
    measurement: string
    tags: string
    start: string
    end: string
    noTags: string
    warnNoTime: string
  },
): string {
  const lines = [
    `${labels.database}: ${scope.database}`,
    `${labels.retention}: ${scope.retention_policy}`,
    `${labels.measurement}: ${scope.measurement}`,
    `${labels.tags}: ${scope.tagsExpr || labels.noTags}`,
    `${labels.start}: ${scope.start_time}`,
    `${labels.end}: ${scope.end_time}`,
  ]
  if (!scope.hasTimeBound) lines.push(labels.warnNoTime)
  return lines.join('\n')
}
