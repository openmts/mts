import assert from 'node:assert/strict'
import test from 'node:test'
import { buildStorageConfigExport, formatStorageExportPretty } from './storageExport.ts'

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
