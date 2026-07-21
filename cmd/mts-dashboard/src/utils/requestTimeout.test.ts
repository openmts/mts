import assert from 'node:assert/strict'
import test from 'node:test'
import { createTimeoutSignal, isAbortError } from './requestTimeout.ts'

test('createTimeoutSignal without timeout uses user signal', () => {
  const c = new AbortController()
  const h = createTimeoutSignal(c.signal, 0)
  assert.equal(h.signal, c.signal)
  assert.equal(h.didTimeout(), false)
  h.cleanup()
})

test('createTimeoutSignal aborts on timeout', async () => {
  const h = createTimeoutSignal(undefined, 20)
  assert.equal(h.signal.aborted, false)
  await new Promise((r) => setTimeout(r, 40))
  assert.equal(h.signal.aborted, true)
  assert.equal(h.didTimeout(), true)
  h.cleanup()
})

test('user abort does not mark timeout', async () => {
  const c = new AbortController()
  const h = createTimeoutSignal(c.signal, 5000)
  c.abort()
  assert.equal(h.signal.aborted, true)
  assert.equal(h.didTimeout(), false)
  h.cleanup()
})

test('isAbortError detects DOMException-like', () => {
  assert.equal(isAbortError({ name: 'AbortError' }), true)
  assert.equal(isAbortError(new Error('x')), false)
})
