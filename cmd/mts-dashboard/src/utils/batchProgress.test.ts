import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyBatchProgressEvent,
  batchProgressPercent,
  emptyBatchProgress,
} from './batchProgress.ts'

test('applyBatchProgressEvent accumulates items and summary', () => {
  let state = emptyBatchProgress()
  let r = applyBatchProgressEvent(state, { type: 'item', index: 1, total: 2, name: 'a', status: 'ok' })
  state = r.next
  assert.equal(state.done, 1)
  assert.equal(state.ok, 1)
  assert.equal(batchProgressPercent(state), 50)
  r = applyBatchProgressEvent(state, { type: 'item', index: 2, total: 2, name: 'b', status: 'skip' })
  state = r.next
  assert.equal(state.skip, 1)
  r = applyBatchProgressEvent(state, {
    type: 'summary',
    ok: true,
    ok_count: 1,
    skip_count: 1,
    fail_count: 0,
    total: 2,
    items: [{ name: 'a', status: 'ok' }, { name: 'b', status: 'skip' }],
  })
  assert.equal(r.summary?.ok_count, 1)
  assert.equal(r.next.fail, 0)
  assert.equal(batchProgressPercent(r.next), 100)
})

test('applyBatchProgressEvent error', () => {
  const r = applyBatchProgressEvent(emptyBatchProgress(), { type: 'error', message: 'boom' })
  assert.equal(r.error, 'boom')
})
