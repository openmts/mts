/** 范围删除结果摘要：优先服务端 path/scope（与 deleteResponse 对齐） */

export interface DeleteAcceptedMeta {
  ok?: boolean
  path?: string
  database?: string
  measurement?: string
}

export function acceptedDeletePath(
  server: DeleteAcceptedMeta | null | undefined,
  clientFallback = '/api/v1/data/delete',
): string {
  const p = String(server?.path || '').trim()
  return p || String(clientFallback || '').trim()
}

export function formatDeleteSuccessMessage(input: {
  server?: DeleteAcceptedMeta | null
  template: string
  format: (template: string, vars: Record<string, string | number>) => string
}): string {
  const path = acceptedDeletePath(input.server)
  const measurement = String(input.server?.measurement || '').trim() || '-'
  const database = String(input.server?.database || '').trim() || 'default'
  return input.format(input.template, { path, measurement, database })
}
