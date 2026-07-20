import type { QueryResultRow } from '@/api/types'
import { formatFieldValue } from '@/utils/fieldValue'

function escapeCSV(v: string): string {
  if (/[",\n\r]/.test(v)) return `"${v.replace(/"/g, '""')}"`
  return v
}

/** 将行结果导出为 CSV 文本 */
export function rowsToCSV(rows: QueryResultRow[]): string {
  if (!rows.length) return ''
  const fieldNames = new Set<string>()
  const tagNames = new Set<string>()
  for (const r of rows) {
    for (const k of Object.keys(r.fields || {})) fieldNames.add(k)
    for (const k of Object.keys(r.tags || {})) tagNames.add(k)
  }
  const tags = [...tagNames].sort()
  const fields = [...fieldNames].sort()
  const header = ['timestamp', 'measurement', ...tags.map((t) => `tag.${t}`), ...fields.map((f) => `field.${f}`)]
  const lines = [header.map(escapeCSV).join(',')]
  for (const r of rows) {
    const cols = [
      String(r.timestamp),
      r.measurement ?? '',
      ...tags.map((t) => r.tags?.[t] ?? ''),
      ...fields.map((f) => formatFieldValue(r.fields?.[f])),
    ]
    lines.push(cols.map((c) => escapeCSV(String(c))).join(','))
  }
  return lines.join('\n')
}

export { downloadText, downloadJSON, stampFilename, triggerDownload } from './download.ts'
