import test from 'node:test'
import assert from 'node:assert/strict'
import { buildPolyline, extractNumericFieldNames, extractSeries } from './chart.ts'

const rows = [
  { series_id: 1, measurement: 'cpu', tags: {}, timestamp: 1, fields: { usage: 1.5, host: 'a' } },
  { series_id: 1, measurement: 'cpu', tags: {}, timestamp: 2, fields: { usage: { float64: 2.5 }, host: 'a' } },
]

test('extractNumericFieldNames', () => {
  assert.deepEqual(extractNumericFieldNames(rows as any), ['usage'])
})

test('extractSeries and polyline', () => {
  const s = extractSeries(rows as any, 'usage')
  assert.equal(s.length, 2)
  const poly = buildPolyline(s, 100, 50)
  assert.ok(poly.path.startsWith('M'))
})
