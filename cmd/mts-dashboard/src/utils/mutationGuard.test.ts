import test from 'node:test'
import assert from 'node:assert/strict'
import {
  isSessionWriteBlocked,
  mutationBlockReason,
  mutationBlockedMessageKey,
  mutationBlockedTitle,
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

test('mutationBlockedMessageKey session vs offline scene keys', () => {
  assert.equal(mutationBlockedMessageKey('session', 'offlineWriteBlocked'), 'sessionMutationBlocked')
  assert.equal(mutationBlockedMessageKey('offline', 'offlineWriteBlocked'), 'offlineWriteBlocked')
  assert.equal(mutationBlockedMessageKey('none', 'offlineOpsBlocked'), 'offlineOpsBlocked')
  assert.equal(mutationBlockedMessageKey(null, ''), 'offlineAdminBlocked')
})

test('mutationBlockedTitle only when blocked', () => {
  const t = (k: string) => `T:${k}`
  assert.equal(mutationBlockedTitle(false, 'session', 'offlineWriteBlocked', t), undefined)
  assert.equal(
    mutationBlockedTitle(true, 'session', 'offlineWriteBlocked', t),
    'T:sessionMutationBlocked',
  )
  assert.equal(
    mutationBlockedTitle(true, 'offline', 'offlineWriteBlocked', t),
    'T:offlineWriteBlocked',
  )
})
