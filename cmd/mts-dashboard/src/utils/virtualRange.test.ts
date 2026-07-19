import assert from 'node:assert/strict'
import test from 'node:test'
import { visibleRange } from './virtualRange.ts'

test('visibleRange clamps and overscans', () => {
  assert.deepEqual(visibleRange(0, 100, 10, 1000, 2), { start: 0, end: 14 })
  assert.deepEqual(visibleRange(500, 100, 10, 100, 2), { start: 48, end: 62 })
  assert.deepEqual(visibleRange(0, 100, 10, 5, 2), { start: 0, end: 5 })
})
