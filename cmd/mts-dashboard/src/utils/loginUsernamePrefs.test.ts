import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clearLoginUsernamePref,
  loadLoginUsernamePref,
  normalizeLoginUsername,
  saveLoginUsernamePref,
} from './loginUsernamePrefs.ts'

test('normalizeLoginUsername trims', () => {
  assert.equal(normalizeLoginUsername('  admin  '), 'admin')
  assert.equal(normalizeLoginUsername(null), '')
})

test('load/save/clear username pref', () => {
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => {
      mem.set(k, v)
    },
    removeItem: (k: string) => {
      mem.delete(k)
    },
  }
  saveLoginUsernamePref(storage, '  ops  ')
  assert.equal(loadLoginUsernamePref(storage), 'ops')
  clearLoginUsernamePref(storage)
  assert.equal(loadLoginUsernamePref(storage), '')
})
