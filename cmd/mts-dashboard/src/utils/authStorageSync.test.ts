import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  AUTH_STORAGE_KEYS,
  AUTH_SESSION_STORAGE_KEYS,
  clearAuthStorage,
  isAuthStorageKey,
  readAuthStorageSnapshot,
} from './authStorageSync.ts'

test('readAuthStorageSnapshot maps keys', () => {
  const store: Record<string, string> = {
    [AUTH_STORAGE_KEYS.token]: 'tok',
    [AUTH_STORAGE_KEYS.user]: 'alice',
    [AUTH_STORAGE_KEYS.role]: 'admin',
    [AUTH_STORAGE_KEYS.expiresAt]: '2026-01-01T00:00:00.000Z',
    [AUTH_STORAGE_KEYS.mustChange]: '1',
  }
  const snap = readAuthStorageSnapshot((k) => store[k])
  assert.equal(snap.token, 'tok')
  assert.equal(snap.user, 'alice')
  assert.equal(snap.role, 'admin')
  assert.equal(snap.expiresAt, '2026-01-01T00:00:00.000Z')
  assert.equal(snap.mustChange, true)
})

test('readAuthStorageSnapshot empty defaults', () => {
  const snap = readAuthStorageSnapshot(() => null)
  assert.deepEqual(snap, {
    token: '',
    user: '',
    role: '',
    expiresAt: '',
    mustChange: false,
  })
})

test('isAuthStorageKey', () => {
  assert.equal(isAuthStorageKey(AUTH_STORAGE_KEYS.token), true)
  assert.equal(isAuthStorageKey('mts_locale'), false)
  assert.equal(isAuthStorageKey(null), false)
})

test('clearAuthStorage removes user session and service credentials', () => {
  const localRemoved: string[] = []
  const sessionRemoved: string[] = []
  clearAuthStorage(
    (key) => localRemoved.push(key),
    (key) => sessionRemoved.push(key),
  )
  assert.deepEqual(localRemoved.sort(), Object.values(AUTH_STORAGE_KEYS).sort())
  assert.deepEqual(sessionRemoved.sort(), Object.values(AUTH_SESSION_STORAGE_KEYS).sort())
})
