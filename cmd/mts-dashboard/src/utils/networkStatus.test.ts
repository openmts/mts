import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isOfflineStatus,
  networkStatusFromOnlineFlag,
  readNavigatorOnline,
} from './networkStatus.ts'

test('networkStatusFromOnlineFlag', () => {
  assert.equal(networkStatusFromOnlineFlag(true), 'online')
  assert.equal(networkStatusFromOnlineFlag(false), 'offline')
  assert.equal(networkStatusFromOnlineFlag(undefined), 'online')
})

test('isOfflineStatus', () => {
  assert.equal(isOfflineStatus('offline'), true)
  assert.equal(isOfflineStatus('online'), false)
})

test('readNavigatorOnline', () => {
  assert.equal(readNavigatorOnline({ onLine: false }), false)
  assert.equal(readNavigatorOnline({ onLine: true }), true)
  assert.equal(readNavigatorOnline(null), true)
  assert.equal(readNavigatorOnline({}), true)
})
