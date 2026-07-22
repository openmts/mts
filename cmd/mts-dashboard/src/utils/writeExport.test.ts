import assert from 'node:assert/strict'
import test from 'node:test'
import { buildWriteDraftExport, buildWriteResultExport } from './writeExport.ts'

test('buildWriteResultExport', () => {
  const out = buildWriteResultExport(
    {
      ok: true,
      message: 'ok',
      mode: 'line',
      server_mode: 'points',
      database: 'db',
      path: '/api/v1/data/write',
      points: 3,
    },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.write.result')
  assert.equal(out.version, 2)
  assert.equal(out.ok, true)
  assert.equal(out.database, 'db')
  assert.equal(out.path, '/api/v1/data/write')
  assert.equal(out.server_mode, 'points')
  assert.equal(out.points, 3)
})

test('buildWriteDraftExport', () => {
  const out = buildWriteDraftExport({ mode: 'line', line_input: 'cpu x=1 1' })
  assert.equal(out.kind, 'mts.write.draft')
  assert.match(out.line_input, /cpu/)
})
