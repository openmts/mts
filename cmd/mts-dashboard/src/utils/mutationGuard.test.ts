import test from 'node:test'
import assert from 'node:assert/strict'
import {
  isSessionWriteBlocked,
  mutationBlockReason,
  shouldBlockMutation,
} from './mutationGuard.ts'

test('isSessionWriteBlocked only critical/expired', () => {
  assert.equal(isSessionWriteBlocked('ok'), false)
  assert.equal(isSessionWriteBlocked('warn'), false)
  assert.equal(isSessionWriteBlocked('critical'), true)
  assert.equal(isSessionWriteBlocked('expired'), true)
  assert.equal(isSessionWriteBlocked('unknown'), false)
  assert.equal(isSessionWriteBlocked(null), false)
})

test('shouldBlockMutation offline or session', () => {
  assert.equal(shouldBlockMutation(true, 'ok'), true)
  assert.equal(shouldBlockMutation(false, 'ok'), false)
  assert.equal(shouldBlockMutation(false, 'warn'), false)
  assert.equal(shouldBlockMutation(false, 'critical'), true)
  assert.equal(shouldBlockMutation(false, 'expired'), true)
  assert.equal(shouldBlockMutation(false), false)
})

test('mutationBlockReason priority offline over session', () => {
  assert.equal(mutationBlockReason(true, 'critical'), 'offline')
  assert.equal(mutationBlockReason(false, 'critical'), 'session')
  assert.equal(mutationBlockReason(false, 'ok'), 'none')
})
