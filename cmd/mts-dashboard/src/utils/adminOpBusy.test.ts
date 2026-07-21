import assert from 'node:assert/strict'
import test from 'node:test'
import {
  adminOpKindLabelKey,
  parseAdminOpBusyPayload,
  parseAdminOpStatusPayload,
  shouldPollAdminOpBusy,
} from './adminOpBusy.ts'

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

test('parseAdminOpStatusPayload idle and busy', () => {
  assert.deepEqual(parseAdminOpStatusPayload({ admin_op_busy: false, op: 'flush', started_at_unix: 9 }), {
    busy: false,
    op: '',
    startedAtUnix: null,
  })
  assert.deepEqual(
    parseAdminOpStatusPayload({ admin_op_busy: true, op: 'compact', started_at_unix: 1700000000 }),
    { busy: true, op: 'compact', startedAtUnix: 1700000000 },
  )
})

test('adminOpKindLabelKey', () => {
  assert.equal(adminOpKindLabelKey('flush'), 'adminOpKindFlush')
  assert.equal(adminOpKindLabelKey('unknown'), 'adminOpKindGeneric')
})
