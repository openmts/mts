import assert from 'node:assert/strict'
import test from 'node:test'
import {
  dismissNotifyItem,
  notifyDisplayText,
  pushNotifyItem,
} from './notifyQueue.ts'

test('pushNotifyItem merges same kind+message within window', () => {
  let nextId = 1
  let items: ReturnType<typeof pushNotifyItem>['items'] = []
  let r = pushNotifyItem(items, { kind: 'error', message: '失败', nextId, now: 1000 })
  items = r.items
  nextId = r.nextId
  assert.equal(r.merged, false)
  assert.equal(items[0].count, 1)
  r = pushNotifyItem(items, { kind: 'error', message: '失败', nextId, now: 1500 })
  items = r.items
  assert.equal(r.merged, true)
  assert.equal(items.length, 1)
  assert.equal(items[0].count, 2)
  assert.equal(notifyDisplayText(items[0]), '失败 (×2)')
})

test('pushNotifyItem drops oldest when over capacity', () => {
  let nextId = 1
  let items: ReturnType<typeof pushNotifyItem>['items'] = []
  for (let i = 0; i < 6; i++) {
    const r = pushNotifyItem(items, {
      kind: 'info',
      message: `m${i}`,
      nextId,
      now: 1000 + i * 10,
      capacity: 3,
      dedupeWindowMs: 0,
    })
    items = r.items
    nextId = r.nextId
  }
  assert.equal(items.length, 3)
  const msgs = items.map((x) => x.message).sort()
  assert.deepEqual(msgs, ['m3', 'm4', 'm5'])
})

test('dismissNotifyItem removes by id', () => {
  const r = pushNotifyItem([], { kind: 'info', message: 'x', nextId: 1, now: 1 })
  assert.equal(dismissNotifyItem(r.items, r.id).length, 0)
})

test('pushNotifyItem keeps action and does not merge different action paths', () => {
  let nextId = 1
  let items: ReturnType<typeof pushNotifyItem>['items'] = []
  let r = pushNotifyItem(items, {
    kind: 'error',
    message: 'busy',
    nextId,
    now: 1000,
    action: { label: 'Ops', path: '/operations#a' },
  })
  items = r.items
  nextId = r.nextId
  assert.equal(items[0].action?.path, '/operations#a')
  r = pushNotifyItem(items, {
    kind: 'error',
    message: 'busy',
    nextId,
    now: 1100,
    action: { label: 'Ops', path: '/operations#b' },
  })
  items = r.items
  assert.equal(r.merged, false)
  assert.equal(items.length, 2)
  r = pushNotifyItem(items, {
    kind: 'error',
    message: 'busy',
    nextId: r.nextId,
    now: 1200,
    action: { label: 'Ops', path: '/operations#a' },
  })
  items = r.items
  assert.equal(r.merged, true)
  assert.equal(items.find((x) => x.id === r.id)?.count, 2)
})
