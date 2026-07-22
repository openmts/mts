import assert from 'node:assert/strict'
import test from 'node:test'
import { buildDeleteResultExport } from './deleteExport.ts'

test('buildDeleteResultExport includes path/scope meta', () => {
  const out = buildDeleteResultExport(
    {
      ok: true,
      message: 'ok',
      path: '/api/v1/data/delete',
      database: 'db1',
      measurement: 'cpu',
      start_time: '1',
      end_time: '2',
    },
    new Date('2026-07-23T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.delete.result')
  assert.equal(out.version, 1)
  assert.equal(out.path, '/api/v1/data/delete')
  assert.equal(out.database, 'db1')
  assert.equal(out.measurement, 'cpu')
  assert.equal(out.start_time, '1')
  assert.equal(out.end_time, '2')
  assert.equal(out.ok, true)
  assert.equal(out.generated_at, '2026-07-23T00:00:00.000Z')
})

test('buildDeleteResultExport empty defaults', () => {
  const out = buildDeleteResultExport(null)
  assert.equal(out.kind, 'mts.delete.result')
  assert.equal(out.path, '/api/v1/data/delete')
  assert.equal(out.database, '')
  assert.equal(out.measurement, '')
  assert.equal(out.ok, null)
})
