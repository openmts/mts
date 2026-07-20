import assert from 'node:assert/strict'
import test from 'node:test'
import { buildWriteDraftExport, buildWriteResultExport } from './writeExport.ts'

test('buildWriteResultExport', () => {
  const out = buildWriteResultExport(
    { ok: true, message: 'ok', mode: 'typed', database: 'db' },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.write.result')
  assert.equal(out.ok, true)
  assert.equal(out.database, 'db')
})

test('buildWriteDraftExport', () => {
  const out = buildWriteDraftExport({ mode: 'line', line_input: 'cpu x=1 1' })
  assert.equal(out.kind, 'mts.write.draft')
  assert.match(out.line_input, /cpu/)
})
