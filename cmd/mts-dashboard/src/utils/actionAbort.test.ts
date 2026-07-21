import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createActionAbort } from './actionAbort.ts'

test('createActionAbort begin/cancel/end', () => {
  const a = createActionAbort()
  const s1 = a.begin()
  assert.equal(a.active(), true)
  assert.equal(s1.aborted, false)
  a.cancel()
  assert.equal(s1.aborted, true)
  assert.equal(a.active(), false)
  const s2 = a.begin()
  a.end()
  assert.equal(a.active(), false)
  assert.equal(s2.aborted, false)
})
