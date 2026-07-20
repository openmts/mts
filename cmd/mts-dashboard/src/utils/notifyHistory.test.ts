import assert from 'node:assert/strict'
import test from 'node:test'
import {
  appendNotifyHistory,
  clearNotifyHistory,
  loadNotifyHistory,
  recordNotifyHistory,
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
  assert.ok(items.length <= 40)
})

test('record/clear notify history storage', () => {
  const s = mem()
  recordNotifyHistory({ kind: 'success', message: 'ok' }, s)
  assert.equal(loadNotifyHistory(s).length, 1)
  clearNotifyHistory(s)
  assert.equal(loadNotifyHistory(s).length, 0)
})
