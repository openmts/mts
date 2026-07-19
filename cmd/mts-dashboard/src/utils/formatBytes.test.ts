import assert from 'node:assert/strict'
import test from 'node:test'
import { formatBytes } from './formatBytes.ts'

test('formatBytes scales', () => {
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(512), '512 B')
  assert.match(formatBytes(2048), /KB/)
  assert.match(formatBytes(2 * 1024 * 1024), /MB/)
})
