import assert from 'node:assert/strict'
import test from 'node:test'
import { DEFAULT_QUERY_PREFS, loadQueryPrefs, parseQueryPrefs, saveQueryPrefs } from './queryPrefs.ts'

test('parseQueryPrefs fills defaults', () => {
  const d = parseQueryPrefs(null)
  assert.equal(d.showChart, DEFAULT_QUERY_PREFS.showChart)
  assert.equal(d.resultColumns.time, true)
  assert.deepEqual(parseQueryPrefs({ showChart: false, resultColumns: { tags: false } }).resultColumns.tags, false)
})

test('load/saveQueryPrefs roundtrip', () => {
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => { mem.set(k, v) },
  }
  saveQueryPrefs(storage, 'k', {
    showChart: false,
    showRawFields: true,
    showHistory: true,
    resultColumns: { time: true, measurement: false, tags: true, fields: true },
  })
  const loaded = loadQueryPrefs(storage, 'k')
  assert.equal(loaded.showChart, false)
  assert.equal(loaded.resultColumns.measurement, false)
})
