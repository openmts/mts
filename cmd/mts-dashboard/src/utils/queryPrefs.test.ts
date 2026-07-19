import assert from 'node:assert/strict'
import test from 'node:test'
import { DEFAULT_QUERY_PREFS, loadQueryPrefs, parseQueryPrefs, saveQueryPrefs } from './queryPrefs.ts'

test('parseQueryPrefs fills defaults', () => {
  assert.deepEqual(parseQueryPrefs(null), DEFAULT_QUERY_PREFS)
  assert.deepEqual(parseQueryPrefs({ showChart: false }), {
    showChart: false,
    showRawFields: false,
    showHistory: false,
  })
})

test('load/saveQueryPrefs roundtrip', () => {
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => { mem.set(k, v) },
  }
  saveQueryPrefs(storage, 'k', { showChart: false, showRawFields: true, showHistory: true })
  assert.deepEqual(loadQueryPrefs(storage, 'k'), {
    showChart: false,
    showRawFields: true,
    showHistory: true,
  })
})
