import test from 'node:test'
import assert from 'node:assert/strict'
import { isOfflineWriteBlocked, shouldBlockOfflineMutation } from './offlineGuard.ts'

test('offline write blocked only when offline true', () => {
  assert.equal(isOfflineWriteBlocked(true), true)
  assert.equal(isOfflineWriteBlocked(false), false)
  assert.equal(isOfflineWriteBlocked(null), false)
  assert.equal(isOfflineWriteBlocked(undefined), false)
  assert.equal(shouldBlockOfflineMutation(true), true)
  assert.equal(shouldBlockOfflineMutation(false), false)
})
