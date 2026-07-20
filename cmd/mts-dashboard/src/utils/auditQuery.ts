/** 审计查询参数构造（纯函数） */

export interface AuditQueryInput {
  userName?: string
  action?: string
  sinceUnix?: number
  untilUnix?: number
  limit?: number
}

export function buildAuditQueryString(input: AuditQueryInput): string {
  const params = new URLSearchParams()
  if (input.userName) params.set('user_name', input.userName)
  if (input.action?.trim()) params.set('action', input.action.trim())
  if (input.sinceUnix) params.set('since_unix', String(input.sinceUnix))
  if (input.untilUnix) params.set('until_unix', String(input.untilUnix))
  const limit = input.limit && input.limit > 0 ? input.limit : 500
  params.set('limit', String(limit))
  return params.toString()
}

export function auditLimitOptions(): number[] {
  return [100, 250, 500, 1000]
}
