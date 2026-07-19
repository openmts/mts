import assert from 'node:assert/strict'
import test from 'node:test'
import {
  historyItemTitle,
  mergeHistoryCap,
  normalizeHistoryItems,
  sortHistoryItems,
} from './queryHistory.ts'

test('sortHistoryItems puts pinned first then newer', () => {
  const items = [
    { id: 'a', at: 1, pinned: false },
    { id: 'b', at: 3, pinned: true },
    { id: 'c', at: 2, pinned: true },
    { id: 'd', at: 4, pinned: false },
  ]
  assert.deepEqual(
    sortHistoryItems(items).map((x) => x.id),
    ['b', 'c', 'd', 'a'],
  )
})

test('historyItemTitle prefers custom name', () => {
  assert.equal(
    historyItemTitle({ name: '  巡检  ', mode: 'rows', form: { database: 'db', measurement: 'm' } }),
    '巡检',
  )
  assert.equal(
    historyItemTitle({ mode: 'columns', form: { database: 'db', measurement: '' } }),
    'db/* · columns',
  )
})

test('mergeHistoryCap keeps pinned when trimming', () => {
  const items = [
    { id: 'p1', at: 1, pinned: true },
    { id: 'p2', at: 2, pinned: true },
    { id: 'n1', at: 10, pinned: false },
    { id: 'n2', at: 9, pinned: false },
    { id: 'n3', at: 8, pinned: false },
  ]
  const capped = mergeHistoryCap(items, 3)
  assert.equal(capped.length, 3)
  assert.ok(capped.every((x) => x.id === 'p1' || x.id === 'p2' || x.id === 'n1'))
  assert.ok(capped.some((x) => x.id === 'p1'))
  assert.ok(capped.some((x) => x.id === 'p2'))
})

test('normalizeHistoryItems drops invalid entries', () => {
  const raw = [
    null,
    { id: 1 },
    {
      id: 'ok',
      at: 1,
      mode: 'rows',
      form: { database: 'd', measurement: 'm', retention_policy: '', start_time: '', end_time: '', fields: '', limit: '10' },
      name: 'n',
      pinned: 1,
    },
  ]
  const got = normalizeHistoryItems(raw)
  assert.equal(got.length, 1)
  assert.equal(got[0].name, 'n')
  assert.equal(got[0].pinned, true)
})
