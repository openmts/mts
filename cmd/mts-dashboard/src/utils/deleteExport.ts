/** 范围删除结果 JSON 导出（与 write/query export 对齐） */

export interface DeleteResultExportInput {
  ok?: boolean | null
  message?: string
  path?: string
  database?: string
  measurement?: string
  start_time?: string | number | null
  end_time?: string | number | null
}

export function buildDeleteResultExport(
  input: DeleteResultExportInput | null | undefined,
  at = new Date(),
): {
  kind: 'mts.delete.result'
  version: 1
  generated_at: string
  ok: boolean | null
  message: string
  path: string
  database: string
  measurement: string
  start_time: string
  end_time: string
} {
  const src = input || {}
  const s = (v: unknown) => (v == null || v === '' ? '' : String(v))
  return {
    kind: 'mts.delete.result',
    version: 1,
    generated_at: at.toISOString(),
    ok: typeof src.ok === 'boolean' ? src.ok : null,
    message: s(src.message),
    path: s(src.path) || '/api/v1/data/delete',
    database: s(src.database),
    measurement: s(src.measurement),
    start_time: s(src.start_time),
    end_time: s(src.end_time),
  }
}
