import assert from 'node:assert/strict'
import test from 'node:test'
import {
  hasOpenNotifyHistory,
  registerOpenNotifyHistory,
  requestOpenNotifyHistory,
} from './notifyHistoryBridge.ts'

test('notifyHistoryBridge registers opens and unregisters', () => {
  const off0 = registerOpenNotifyHistory(() => {})
  off0()
  assert.equal(hasOpenNotifyHistory(), false)
  assert.equal(requestOpenNotifyHistory(), false)

  let n = 0
  const off = registerOpenNotifyHistory(() => {
    n += 1
  })
  assert.equal(hasOpenNotifyHistory(), true)
  assert.equal(requestOpenNotifyHistory(), true)
  assert.equal(n, 1)
  off()
  assert.equal(hasOpenNotifyHistory(), false)
  assert.equal(requestOpenNotifyHistory(), false)
})
