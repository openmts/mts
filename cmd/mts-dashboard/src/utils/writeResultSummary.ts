/** 写入结果摘要：优先使用服务端 accepted points/path（与 writeResponse 对齐） */

export interface WriteAcceptedMeta {
  ok?: boolean
  points?: number
  path?: string
  mode?: string
}

export function acceptedWritePoints(
  server: WriteAcceptedMeta | null | undefined,
  clientFallback: number,
): number {
  const n = Number(server?.points)
  if (Number.isFinite(n) && n >= 0) return Math.trunc(n)
  const fb = Number(clientFallback)
  return Number.isFinite(fb) && fb >= 0 ? Math.trunc(fb) : 0
}

export function acceptedWritePath(
  server: WriteAcceptedMeta | null | undefined,
  clientFallback: string,
): string {
  const p = String(server?.path || '').trim()
  if (p) return p
  return String(clientFallback || '').trim()
}

export function formatWriteSuccessMessage(input: {
  mode: 'typed' | 'points'
  server?: WriteAcceptedMeta | null
  clientCount: number
  clientPath: string
  typedTemplate: string
  pointsTemplate: string
  /** 可选：带 path 的 TypedBatch 模板（{count}/{path}） */
  typedWithPathTemplate?: string
  format: (template: string, vars: Record<string, string | number>) => string
}): string {
  const count = acceptedWritePoints(input.server, input.clientCount)
  const path = acceptedWritePath(input.server, input.clientPath)
  if (input.mode === 'typed') {
    if (path && input.typedWithPathTemplate) {
      return input.format(input.typedWithPathTemplate, { count, path })
    }
    return input.format(input.typedTemplate, { count })
  }
  return input.format(input.pointsTemplate, { count, path })
}
