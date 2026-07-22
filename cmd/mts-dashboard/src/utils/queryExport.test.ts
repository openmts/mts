import assert from 'node:assert/strict'
import test from 'node:test'
import { buildQueryResultExport } from './queryExport.ts'

test('buildQueryResultExport includes server scope meta', () => {
  const out = buildQueryResultExport(
    {
      mode: 'rows',
      path: '/api/v1/data/query/rows',
      database: 'default',
      measurement: 'cpu',
      row_count: 2,
      series_count: 0,
      query: { database: 'default', measurement: 'cpu' },
      rows: [{ timestamp: 1 }],
    },
    new Date('2026-07-23T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.query.result')
  assert.equal(out.version, 1)
  assert.equal(out.path, '/api/v1/data/query/rows')
  assert.equal(out.database, 'default')
  assert.equal(out.measurement, 'cpu')
  assert.equal(out.row_count, 2)
  assert.equal(out.rows?.length, 1)
})

test('buildQueryResultExport empty defaults', () => {
  const out = buildQueryResultExport(null)
  assert.equal(out.mode, '')
  assert.equal(out.row_count, null)
  assert.equal(out.rows, null)
})
