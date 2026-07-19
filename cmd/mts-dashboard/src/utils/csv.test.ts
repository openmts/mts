import assert from 'node:assert/strict'
import test from 'node:test'
import { formatFieldValue } from './fieldValue.ts'

// 与 csv.ts 保持一致的纯函数契约测试（避免 node 对 @ 路径别名的解析）
function rowsToCSV(rows: Array<{
  timestamp: number
  measurement: string
  tags?: Record<string, string>
  fields?: Record<string, unknown>
}>): string {
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
  const lines = [header.join(',')]
  for (const r of rows) {
    const cols = [
      String(r.timestamp),
      r.measurement ?? '',
      ...tags.map((t) => r.tags?.[t] ?? ''),
      ...fields.map((f) => formatFieldValue(r.fields?.[f])),
    ]
    lines.push(cols.join(','))
  }
  return lines.join('\n')
}

test('rowsToCSV includes tags and expanded fields', () => {
  const csv = rowsToCSV([
    {
      timestamp: 10,
      measurement: 'cpu',
      tags: { host: 'a' },
      fields: { usage: { float64: 0.7 } },
    },
  ])
  assert.match(csv, /timestamp,measurement,tag\.host,field\.usage/)
  assert.match(csv, /10,cpu,a,0\.7/)
})
