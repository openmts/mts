/** 写入页结果与草稿导出（纯函数） */

export interface WriteResultExportInput {
  ok?: boolean | null
  message?: string
  mode?: string
  database?: string
  retention_policy?: string
  sync?: boolean
  use_points_typed?: boolean
  path?: string
}

export function buildWriteResultExport(
  input: WriteResultExportInput | null | undefined,
  at = new Date(),
): {
  kind: 'mts.write.result'
  version: 1
  generated_at: string
  ok: boolean | null
  message: string
  mode: string
  database: string
  retention_policy: string
  sync: boolean
  use_points_typed: boolean
  path: string
} {
  const src = input || {}
  return {
    kind: 'mts.write.result',
    version: 1,
    generated_at: at.toISOString(),
    ok: typeof src.ok === 'boolean' ? src.ok : null,
    message: src.message || '',
    mode: src.mode || '',
    database: src.database || '',
    retention_policy: src.retention_policy || '',
    sync: Boolean(src.sync),
    use_points_typed: Boolean(src.use_points_typed),
    path: src.path || '',
  }
}

export function buildWriteDraftExport(
  input: {
    mode?: string
    database?: string
    retention_policy?: string
    line_input?: string
    form_rows?: unknown[]
    typed?: Record<string, unknown> | null
  },
  at = new Date(),
): {
  kind: 'mts.write.draft'
  version: 1
  generated_at: string
  mode: string
  database: string
  retention_policy: string
  line_input: string
  form_rows: unknown[]
  typed: Record<string, unknown> | null
} {
  return {
    kind: 'mts.write.draft',
    version: 1,
    generated_at: at.toISOString(),
    mode: input.mode || '',
    database: input.database || '',
    retention_policy: input.retention_policy || '',
    line_input: input.line_input || '',
    form_rows: Array.isArray(input.form_rows) ? input.form_rows : [],
    typed: input.typed ?? null,
  }
}
