import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildHistoryExport,
  mergeImportedHistory,
  parseHistoryImport,
} from './queryHistoryIO.ts'
import type { QueryHistoryRecord } from './queryHistory.ts'

const sample = (id: string, at: number, pinned = false): QueryHistoryRecord => ({
  id,
  at,
  mode: 'rows',
  pinned,
  form: {
    database: 'db',
    retention_policy: 'autogen',
    measurement: 'm',
    start_time: '1',
    end_time: '2',
    fields: '',
    limit: '100',
  },
})

test('buildHistoryExport wraps items', () => {
  const payload = buildHistoryExport([sample('a', 1)], 99)
  assert.equal(payload.version, 1)
  assert.equal(payload.exported_at, 99)
  assert.equal(payload.items.length, 1)
  assert.equal(payload.items[0].id, 'a')
})

test('parseHistoryImport accepts array and object', () => {
  const a = parseHistoryImport([sample('a', 1)])
  assert.equal(a.ok, true)
  if (a.ok) assert.equal(a.items.length, 1)
  const b = parseHistoryImport({ items: [sample('b', 2)] })
  assert.equal(b.ok, true)
  const c = parseHistoryImport({ foo: 1 })
  assert.equal(c.ok, false)
})

test('mergeImportedHistory merge and replace', () => {
  const cur = [sample('a', 1), sample('b', 2, true)]
  const inc = [sample('a', 10), sample('c', 3)]
  const merged = mergeImportedHistory(cur, inc, { merge: true, max: 10 })
  const a = merged.find((x) => x.id === 'a')
  assert.equal(a?.at, 10)
  assert.ok(merged.some((x) => x.id === 'b'))
  assert.ok(merged.some((x) => x.id === 'c'))
  const replaced = mergeImportedHistory(cur, inc, { merge: false, max: 10 })
  assert.equal(replaced.length, 2)
  assert.ok(!replaced.some((x) => x.id === 'b'))
})
