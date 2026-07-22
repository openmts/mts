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

test('applyBatchProgressEvent cancelled summary', () => {
  let state = emptyBatchProgress()
  state = applyBatchProgressEvent(state, { type: 'item', index: 1, total: 3, name: 'a', status: 'ok' }).next
  const r = applyBatchProgressEvent(state, {
    type: 'summary',
    ok: true,
    ok_count: 1,
    skip_count: 0,
    fail_count: 0,
    total: 3,
    cancelled: true,
    message: 'context canceled',
    items: [{ name: 'a', status: 'ok' }],
  })
  assert.equal(r.summary?.cancelled, true)
  assert.equal(r.next.ok, 1)
  assert.equal(r.next.done, 3)
})

test('applyBatchProgressEvent summary carries admin op fields', () => {
  const r = applyBatchProgressEvent(emptyBatchProgress(), {
    type: 'summary',
    ok: true,
    ok_count: 1,
    skip_count: 0,
    fail_count: 0,
    total: 1,
    items: [{ name: 'a', status: 'ok' }],
    admin_op_busy: false,
    op: 'compact',
    started_at_unix: 0,
    last: { ok: false, op: 'compact', error: 'e2e disk full', finished_at_unix: 1 },
  })
  assert.equal(r.summary?.admin_op_busy, false)
  assert.equal(r.summary?.op, 'compact')
  assert.equal((r.summary?.last as { error?: string } | undefined)?.error, 'e2e disk full')
})

test('applyBatchProgressEvent summary carries path', () => {
  const r = applyBatchProgressEvent(emptyBatchProgress(), {
    type: 'summary',
    ok: true,
    path: '/api/v1/users/batch-disabled',
    ok_count: 1,
    skip_count: 0,
    fail_count: 0,
    items: [],
  })
  assert.equal(r.summary?.path, '/api/v1/users/batch-disabled')
})
