import assert from 'node:assert/strict'
import test from 'node:test'
import {
  extractNumericFieldNames,
  extractSeries,
  extractMultiSeries,
  polyline,
  polylineInBounds,
  boundsOfSeries,
} from './chart.ts'
import type { QueryResultRow } from '../api/types.ts'

const rows: QueryResultRow[] = [
  { series_id: 1, measurement: 'cpu', tags: { host: 'a' }, timestamp: 1, fields: { usage: { float64: 0.1 } } },
  { series_id: 2, measurement: 'cpu', tags: { host: 'b' }, timestamp: 1, fields: { usage: { float64: 0.5 } } },
  { series_id: 1, measurement: 'cpu', tags: { host: 'a' }, timestamp: 2, fields: { usage: { float64: 0.2 } } },
  { series_id: 2, measurement: 'cpu', tags: { host: 'b' }, timestamp: 2, fields: { usage: 0.6 } },
]

test('extractNumericFieldNames', () => {
  assert.deepEqual(extractNumericFieldNames(rows), ['usage'])
})

test('extractSeries and polyline', () => {
  // 默认取点数最多/排序后第一 series（a 与 b 各 2 点，按 key 排序 a 在前）
  const pts = extractSeries(rows, 'usage')
  assert.equal(pts.length, 2)
  assert.ok(polyline(pts, 200, 80).startsWith('M'))
})

test('extractMultiSeries splits by tags', () => {
  const multi = extractMultiSeries(rows, 'usage', 8)
  assert.equal(multi.length, 2)
  assert.equal(multi.find((s) => s.key.includes('host=a'))?.points.length, 2)
  assert.equal(multi.find((s) => s.key.includes('host=b'))?.points.length, 2)
  const b = boundsOfSeries(multi)
  assert.ok(b)
  assert.ok(polylineInBounds(multi[0].points, b!, 200, 80).includes('L'))
})
