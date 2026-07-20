import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_WRITE_PREFS,
  loadWritePrefs,
  parseWritePrefs,
  saveWritePrefs,
} from './writePrefs.ts'

test('parseWritePrefs defaults to typed', () => {
  assert.deepEqual(parseWritePrefs(null), DEFAULT_WRITE_PREFS)
  assert.equal(parseWritePrefs({}).writeMode, 'typed')
  assert.equal(parseWritePrefs({ writeMode: 'form' }).writeMode, 'form')
  assert.equal(parseWritePrefs({ writeMode: 'nope' }).writeMode, 'typed')
})

test('load/save write prefs roundtrip', () => {
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => {
      mem.set(k, v)
    },
  }
  saveWritePrefs(storage, { writeMode: 'line', usePointsTyped: false, syncWrite: true })
  const loaded = loadWritePrefs(storage)
  assert.deepEqual(loaded, { writeMode: 'line', usePointsTyped: false, syncWrite: true })
})
