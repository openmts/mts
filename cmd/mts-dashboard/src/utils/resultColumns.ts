/** 查询结果表列可见性 */

export type ResultColumnKey = 'time' | 'measurement' | 'tags' | 'fields'
export type ResultColumnLocale = 'zh' | 'en'

export const RESULT_COLUMN_KEYS: ResultColumnKey[] = ['time', 'measurement', 'tags', 'fields']

/** @deprecated 默认中文标签；展示请用 resultColumnLabel */
export const RESULT_COLUMN_LABELS: Record<ResultColumnKey, string> = {
  time: '时间',
  measurement: 'Measurement',
  tags: 'Tags',
  fields: 'Fields',
}

const LABELS: Record<ResultColumnKey, Record<ResultColumnLocale, string>> = {
  time: { zh: '时间', en: 'Time' },
  measurement: { zh: 'Measurement', en: 'Measurement' },
  tags: { zh: 'Tags', en: 'Tags' },
  fields: { zh: 'Fields', en: 'Fields' },
}

export function resultColumnLabel(key: ResultColumnKey, locale: ResultColumnLocale = 'zh'): string {
  return LABELS[key][locale === 'en' ? 'en' : 'zh']
}

export type ResultColumnVisibility = Record<ResultColumnKey, boolean>

export const DEFAULT_RESULT_COLUMNS: ResultColumnVisibility = {
  time: true,
  measurement: true,
  tags: true,
  fields: true,
}

export function parseResultColumns(raw: unknown): ResultColumnVisibility {
  const base = { ...DEFAULT_RESULT_COLUMNS }
  if (!raw || typeof raw !== 'object') return base
  const o = raw as Record<string, unknown>
  for (const k of RESULT_COLUMN_KEYS) {
    if (typeof o[k] === 'boolean') base[k] = o[k]
  }
  // 至少保留一列，避免空表
  if (!RESULT_COLUMN_KEYS.some((k) => base[k])) {
    base.time = true
  }
  return base
}

export function visibleColumnCount(cols: ResultColumnVisibility): number {
  return RESULT_COLUMN_KEYS.filter((k) => cols[k]).length
}

/** 用于 grid-cols-N 的安全列数 1..4 */
export function gridColClass(cols: ResultColumnVisibility): string {
  const n = Math.min(4, Math.max(1, visibleColumnCount(cols)))
  return (
    {
      1: 'grid-cols-1',
      2: 'grid-cols-2',
      3: 'grid-cols-3',
      4: 'grid-cols-4',
    } as const
  )[n as 1 | 2 | 3 | 4]
}

export function toggleResultColumn(
  cols: ResultColumnVisibility,
  key: ResultColumnKey,
): ResultColumnVisibility {
  const next = { ...cols, [key]: !cols[key] }
  if (!RESULT_COLUMN_KEYS.some((k) => next[k])) {
    // 禁止全部关闭
    return cols
  }
  return next
}
