import assert from 'node:assert/strict'
import test from 'node:test'
import {
  ADMIN_OP_BUSY_OPS_PATH,
  adminHeavyBusyOpFromError,
  adminOpBusyOpenAction,
  adminOpKindLabelKey,
  formatAdminOpElapsed,
  isAdminHeavyBusyError,
  isAdminHeavyBusyMessage,
  joinAdminOpChip,
  parseAdminHeavyBusyOp,
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

test('formatAdminOpElapsed', () => {
  assert.equal(formatAdminOpElapsed(null), '—')
  assert.equal(formatAdminOpElapsed(0), '—')
  const started = 1_700_000_000
  assert.equal(formatAdminOpElapsed(started, started * 1000 + 4500), '4s')
  assert.equal(formatAdminOpElapsed(started, started * 1000 + 65_000), '1m05s')
})

test('joinAdminOpChip', () => {
  assert.equal(joinAdminOpChip('busy'), 'busy')
  assert.equal(joinAdminOpChip('busy', 'Flush'), 'busy: Flush')
})

test('isAdminHeavyBusyMessage and error', () => {
  assert.equal(isAdminHeavyBusyMessage('admin heavy operation already in progress'), true)
  assert.equal(isAdminHeavyBusyMessage('rate limit'), false)
  assert.equal(
    isAdminHeavyBusyError({
      name: 'APIClientError',
      code: 'resource_exhausted',
      status: 429,
      message: 'admin heavy operation already in progress',
    }),
    true,
  )
  assert.equal(
    isAdminHeavyBusyError({ code: 'resource_exhausted', message: 'query limit' }),
    false,
  )
})

test('parseAdminHeavyBusyOp', () => {
  assert.equal(parseAdminHeavyBusyOp('admin heavy operation already in progress: flush'), 'flush')
  assert.equal(parseAdminHeavyBusyOp('admin heavy operation already in progress'), '')
  assert.equal(
    adminHeavyBusyOpFromError({
      code: 'resource_exhausted',
      status: 429,
      message: 'admin heavy operation already in progress: data_snapshot',
    }),
    'data_snapshot',
  )
})

test('adminOpBusyOpenAction', () => {
  assert.equal(ADMIN_OP_BUSY_OPS_PATH, '/operations#ops-status-strip')
  assert.deepEqual(adminOpBusyOpenAction('打开运维'), {
    label: '打开运维',
    path: ADMIN_OP_BUSY_OPS_PATH,
  })
  assert.equal(adminOpBusyOpenAction('').path, ADMIN_OP_BUSY_OPS_PATH)
})

test('structured adminOpBusy flag and op', () => {
  assert.equal(
    isAdminHeavyBusyError({
      code: 'resource_exhausted',
      status: 429,
      message: 'anything',
      adminOpBusy: true,
      op: 'flush',
    }),
    true,
  )
  assert.equal(
    adminHeavyBusyOpFromError({
      code: 'resource_exhausted',
      status: 429,
      adminOpBusy: true,
      op: 'compact',
      message: 'admin heavy operation already in progress: flush',
    }),
    'compact',
  )
})

