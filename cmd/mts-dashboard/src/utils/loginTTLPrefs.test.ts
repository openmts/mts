import assert from 'node:assert/strict'
import test from 'node:test'
import { loadLoginTTLPref, saveLoginTTLPref } from './loginTTLPrefs.ts'

test('login ttl pref roundtrip and clear', () => {
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
  assert.equal(loadLoginTTLPref(storage), '')
  saveLoginTTLPref(storage, '3600')
  assert.equal(loadLoginTTLPref(storage), '3600')
  saveLoginTTLPref(storage, '  ')
  assert.equal(loadLoginTTLPref(storage), '')
})
