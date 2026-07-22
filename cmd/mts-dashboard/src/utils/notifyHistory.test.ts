import assert from 'node:assert/strict'
import test from 'node:test'
import {
  appendNotifyHistory,
  clearNotifyHistory,
  filterNotifyHistory,
  filterNotifyHistoryByKind,
  filterNotifyHistoryByTime,
  loadNotifyHistory,
  notifyHistoryRangeBounds,
  recordNotifyHistory,
  searchNotifyHistory,
} from './notifyHistory.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
}

test('appendNotifyHistory newest first and caps', () => {
  let items = appendNotifyHistory([], { kind: 'info', message: 'a', at: 1 })
  items = appendNotifyHistory(items, { kind: 'error', message: 'b', at: 2 })
  assert.equal(items[0]?.message, 'b')
  assert.equal(items[1]?.message, 'a')
  for (let i = 0; i < 50; i++) {
    items = appendNotifyHistory(items, { kind: 'info', message: `m${i}`, at: 10 + i })
  }
  assert.ok(items.length <= 200)
})

test('record/clear notify history storage', () => {
  const s = mem()
  recordNotifyHistory({ kind: 'success', message: 'ok' }, s)
  assert.equal(loadNotifyHistory(s).length, 1)
  clearNotifyHistory(s)
  assert.equal(loadNotifyHistory(s).length, 0)
})

test('filterNotifyHistoryByKind', () => {
  const items = [
    { id: '1', kind: 'info' as const, message: 'a', count: 1, at: 1 },
    { id: '2', kind: 'error' as const, message: 'b', count: 1, at: 2 },
  ]
  assert.equal(filterNotifyHistoryByKind(items, 'all').length, 2)
  assert.equal(filterNotifyHistoryByKind(items, 'error').length, 1)
  assert.equal(filterNotifyHistoryByKind(items, 'error')[0]?.message, 'b')
})

test('searchNotifyHistory matches message and kind', () => {
  const items = [
    { id: '1', kind: 'info' as const, message: 'flush ok', count: 1, at: 1 },
    { id: '2', kind: 'error' as const, message: 'compact fail', count: 1, at: 2 },
  ]
  assert.equal(searchNotifyHistory(items, 'flush').length, 1)
  assert.equal(searchNotifyHistory(items, 'ERROR').length, 1)
  assert.equal(searchNotifyHistory(items, '').length, 2)
})

test('filterNotifyHistory combines kind and query', () => {
  const items = [
    { id: '1', kind: 'error' as const, message: 'a fail', count: 1, at: 1 },
    { id: '2', kind: 'error' as const, message: 'b ok', count: 1, at: 2 },
    { id: '3', kind: 'info' as const, message: 'a fail', count: 1, at: 3 },
  ]
  const got = filterNotifyHistory(items, { kind: 'error', query: 'fail' })
  assert.equal(got.length, 1)
  assert.equal(got[0]?.id, '1')
})

test('filterNotifyHistoryByTime inclusive bounds', () => {
  const items = [
    { id: '1', kind: 'info' as const, message: 'a', count: 1, at: 100 },
    { id: '2', kind: 'info' as const, message: 'b', count: 1, at: 200 },
    { id: '3', kind: 'info' as const, message: 'c', count: 1, at: 300 },
  ]
  assert.deepEqual(
    filterNotifyHistoryByTime(items, { sinceMs: 200, untilMs: 300 }).map((x) => x.id),
    ['2', '3'],
  )
  assert.equal(filterNotifyHistoryByTime(items, {}).length, 3)
})

test('notifyHistoryRangeBounds', () => {
  const b = notifyHistoryRangeBounds('1h', 10_000_000)
  assert.equal(b.untilMs, 10_000_000)
  assert.equal(b.sinceMs, 10_000_000 - 3600_000)
  assert.deepEqual(notifyHistoryRangeBounds('all', 1), { sinceMs: null, untilMs: null })
})

test('filterNotifyHistory with time + kind + query', () => {
  const items = [
    { id: '1', kind: 'error' as const, message: 'old fail', count: 1, at: 100 },
    { id: '2', kind: 'error' as const, message: 'new fail', count: 1, at: 500 },
    { id: '3', kind: 'info' as const, message: 'new fail', count: 1, at: 600 },
  ]
  const got = filterNotifyHistory(items, {
    kind: 'error',
    query: 'fail',
    sinceMs: 400,
    untilMs: 700,
  })
  assert.equal(got.length, 1)
  assert.equal(got[0]?.id, '2')
})

test('appendNotifyHistory keeps action and search matches path', () => {
  const items = appendNotifyHistory([], {
    kind: 'error',
    message: 'busy',
    at: 1,
    actionLabel: '打开运维',
    actionPath: '/operations#ops-status-strip',
  })
  assert.equal(items[0]?.actionPath, '/operations#ops-status-strip')
  assert.equal(items[0]?.actionLabel, '打开运维')
  assert.equal(searchNotifyHistory(items, 'ops-status').length, 1)
  const s = mem()
  recordNotifyHistory({
    kind: 'error',
    message: 'busy2',
    actionLabel: 'Open',
    actionPath: '/operations#ops-status-strip',
  }, s)
  const loaded = loadNotifyHistory(s)
  assert.equal(loaded[0]?.actionPath, '/operations#ops-status-strip')
})

