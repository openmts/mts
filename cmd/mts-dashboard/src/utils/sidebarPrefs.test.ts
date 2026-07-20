import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_SIDEBAR_PREFS,
  loadSidebarPrefs,
  parseSidebarPrefs,
  saveSidebarPrefs,
} from './sidebarPrefs.ts'

test('parseSidebarPrefs defaults', () => {
  assert.deepEqual(parseSidebarPrefs(null), DEFAULT_SIDEBAR_PREFS)
  assert.equal(parseSidebarPrefs({ collapsed: true }).collapsed, true)
})

test('load/save sidebar prefs', () => {
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => {
      mem.set(k, v)
    },
  }
  saveSidebarPrefs(storage, { collapsed: true })
  assert.equal(loadSidebarPrefs(storage).collapsed, true)
})
