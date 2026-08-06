import type { QueryResultRow } from '@/api/types'
import { formatFieldValue } from '@/utils/fieldValue'
import { escapeCSVCell } from './csvEscape.ts'

/** 收集 query 行的 tag/field 列集合 */
export function collectQueryCSVColumns(rows: QueryResultRow[]): { tags: string[]; fields: string[] } {
  const fieldNames = new Set<string>()
  const tagNames = new Set<string>()
  for (const r of rows) {
    for (const k of Object.keys(r.fields || {})) fieldNames.add(k)
    for (const k of Object.keys(r.tags || {})) tagNames.add(k)
  }
  return { tags: [...tagNames].sort(), fields: [...fieldNames].sort() }
}

export function queryCSVHeader(tags: string[], fields: string[]): string {
  const header = ['timestamp', 'measurement', ...tags.map((t) => `tag.${t}`), ...fields.map((f) => `field.${f}`)]
  return header.map(escapeCSVCell).join(',')
}

export function queryCSVRow(r: QueryResultRow, tags: string[], fields: string[]): string {
  const cols = [
    String(r.timestamp),
    r.measurement ?? '',
    ...tags.map((t) => r.tags?.[t] ?? ''),
    ...fields.map((f) => formatFieldValue(r.fields?.[f])),
  ]
  return cols.map(escapeCSVCell).join(',')
}

/** 将行结果导出为 CSV 文本 */
export function rowsToCSV(rows: QueryResultRow[]): string {
  if (!rows.length) return ''
  const { tags, fields } = collectQueryCSVColumns(rows)
  const lines = [queryCSVHeader(tags, fields)]
  for (const r of rows) lines.push(queryCSVRow(r, tags, fields))
  return lines.join('\n')
}

export { downloadText, downloadJSON, stampFilename, triggerDownload } from './download.ts'
