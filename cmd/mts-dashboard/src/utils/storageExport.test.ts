import assert from 'node:assert/strict'
import test from 'node:test'
import { buildStorageConfigExport, formatStorageExportPretty, summarizeStorageExport } from './storageExport.ts'

test('buildStorageConfigExport wraps payload', () => {
  const out = buildStorageConfigExport({ a: 1 }, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.storage.export')
  assert.equal(out.export.a, 1)
})

test('formatStorageExportPretty', () => {
  const text = formatStorageExportPretty({ x: true })
  assert.match(text, /mts.storage.export/)
  assert.match(text, /"x": true/)
})

test('summarizeStorageExport counts users and grants', () => {
  const s = summarizeStorageExport({
    generated_at: '2026-07-23T00:00:00Z',
    config: { a: 1, b: 2 },
    health: { healthy: true, ready: false, reasons: ['x'] },
    users: [{ name: 'a' }, { name: 'b' }],
    grants: { a: [{}, {}], b: [{}] },
  }, '/api/v1/admin/storage/export')
  assert.equal(s?.user_count, 2)
  assert.equal(s?.grant_user_count, 2)
  assert.equal(s?.grant_total, 3)
  assert.equal(s?.healthy, true)
  assert.equal(s?.ready, false)
  assert.equal(s?.reason_count, 1)
  assert.equal(s?.config_keys, 2)
  assert.equal(s?.path, '/api/v1/admin/storage/export')
})

test('summarizeStorageExport returns null for empty', () => {
  assert.equal(summarizeStorageExport(null), null)
  assert.equal(summarizeStorageExport(undefined), null)
})
