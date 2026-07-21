import assert from 'node:assert/strict'
import test from 'node:test'
import { parseAdminOpBusyPayload, shouldPollAdminOpBusy } from './adminOpBusy.ts'

test('shouldPollAdminOpBusy only for authenticated admin', () => {
  assert.equal(shouldPollAdminOpBusy(true, true), true)
  assert.equal(shouldPollAdminOpBusy(true, false), false)
  assert.equal(shouldPollAdminOpBusy(false, true), false)
})

test('parseAdminOpBusyPayload', () => {
  assert.equal(parseAdminOpBusyPayload({ admin_op_busy: true }), true)
  assert.equal(parseAdminOpBusyPayload({ admin_op_busy: false }), false)
  assert.equal(parseAdminOpBusyPayload(null), false)
})
