/** 空值展示（与 i18n emptyValue 对齐） */
export const EMPTY_DISPLAY = '—'

/** 将查询返回的 FieldValue / 原始标量格式化为可读文本 */
export function formatFieldValue(raw: unknown): string {
  if (raw === null || raw === undefined) return EMPTY_DISPLAY
  if (typeof raw === 'number' || typeof raw === 'boolean') return String(raw)
  if (typeof raw === 'string') return raw
  if (typeof raw === 'object') {
    const o = raw as Record<string, unknown>
    if (typeof o.float64 === 'number' && Number.isFinite(o.float64)) return String(o.float64)
    if (typeof o.int64 === 'number' && Number.isFinite(o.int64)) return String(o.int64)
    if (typeof o.string === 'string') return o.string
    if (typeof o.bool === 'boolean') return String(o.bool)
  }
  try {
    return JSON.stringify(raw)
  } catch {
    return String(raw)
  }
}

export function formatFieldsMap(fields: Record<string, unknown> | null | undefined): string {
  if (!fields || !Object.keys(fields).length) return EMPTY_DISPLAY
  return Object.entries(fields)
    .map(([k, v]) => `${k}=${formatFieldValue(v)}`)
    .join(', ')
}
