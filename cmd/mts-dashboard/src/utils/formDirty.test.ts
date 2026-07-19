import assert from 'node:assert/strict'
import test from 'node:test'
import { isDirty, snapshotForm, stableStringify } from './formDirty.ts'

test('stableStringify ignores key order', () => {
  assert.equal(stableStringify({ b: 1, a: 2 }), stableStringify({ a: 2, b: 1 }))
})

test('isDirty detects change', () => {
  const base = snapshotForm({ x: 1, y: 'a' })
  assert.equal(isDirty(base, { y: 'a', x: 1 }), false)
  assert.equal(isDirty(base, { x: 2, y: 'a' }), true)
})

test('snapshotForm detaches reference', () => {
  const src = { a: 1 }
  const snap = snapshotForm(src)
  src.a = 2
  assert.equal(snap.a, 1)
})
