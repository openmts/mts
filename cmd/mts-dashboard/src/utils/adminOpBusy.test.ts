import assert from 'node:assert/strict'
import test from 'node:test'
import {
  ADMIN_OP_BUSY_OPS_PATH,
  ADMIN_OP_POLL_BUSY_MS,
  ADMIN_OP_POLL_FAIL_BACKOFF_MS,
  ADMIN_OP_POLL_IDLE_MS,
  adminHeavyBusyOpFromError,
  adminOpBusyOpenAction,
  adminOpKindLabelKey,
  adminOpPollIntervalMs,
  formatAdminOpElapsed,
  isAdminHeavyBusyError,
  isAdminHeavyBusyMessage,
  joinAdminOpChip,
  nextAdminOpFailStreak,
  commandAdminOpRefreshFeedback,
  resolveAdminBusyNotify,
  parseAdminBusyFromHeaders,
  parseAdminHeavyBusyOp,
  parseAdminOpBusyPayload,
  parseAdminOpStatusPayload,
  parseAdminHeavyLast,
  formatAdminHeavyLastSummary,
  shouldShowAdminOpLastBanner,
  readDismissedAdminOpLastFinishedAt,
  writeDismissedAdminOpLastFinishedAt,
  adminOpLastToneClass,
  adminOpLastBannerSurfaceClass,
  adminOpLastChipSurfaceClass,
  commandAdminOpLastDismissFeedback,
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
    last: null,
  })
  assert.deepEqual(
    parseAdminOpStatusPayload({ admin_op_busy: true, op: 'compact', started_at_unix: 1700000000 }),
    { busy: true, op: 'compact', startedAtUnix: 1700000000, last: null },
  )
  const withLast = parseAdminOpStatusPayload({
    admin_op_busy: false,
    last: {
      op: 'flush',
      ok: true,
      finished_at_unix: 100,
      started_at_unix: 90,
      duration_ms: 10000,
    },
  })
  assert.equal(withLast.busy, false)
  assert.equal(withLast.last?.op, 'flush')
  assert.equal(withLast.last?.ok, true)
  assert.equal(withLast.last?.durationMs, 10000)
})

test('parseAdminHeavyLast and formatAdminHeavyLastSummary', () => {
  assert.equal(parseAdminHeavyLast(null), null)
  const last = parseAdminHeavyLast({
    op: 'compact',
    ok: false,
    error: 'disk full',
    duration_ms: 2500,
  })
  assert.equal(last?.ok, false)
  assert.match(formatAdminHeavyLastSummary(last, 'Compact'), /fail/)
  assert.match(formatAdminHeavyLastSummary(last, 'Compact'), /disk full/)
  assert.match(formatAdminHeavyLastSummary(last, 'Compact'), /2\.5s/)
  assert.match(
    formatAdminHeavyLastSummary(
      { op: 'x', ok: true, error: '', startedAtUnix: 1, finishedAtUnix: 1, durationMs: 40 },
      'X',
    ),
    /40ms/,
  )
  assert.match(
    formatAdminHeavyLastSummary(
      { op: 'flush', ok: true, error: '', startedAtUnix: 1, finishedAtUnix: 2, durationMs: 3000 },
      'Flush',
    ),
    /ok/,
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

test('parseAdminBusyFromHeaders', () => {
  const h = (name: string) => {
    const m: Record<string, string> = {
      'X-MTS-Admin-Op-Busy': 'true',
      'X-MTS-Admin-Op': 'flush',
    }
    return m[name] || null
  }
  assert.deepEqual(parseAdminBusyFromHeaders(h), { busy: true, op: 'flush' })
  assert.deepEqual(parseAdminBusyFromHeaders(() => null), { busy: false, op: '' })
})

test('adminOpPollIntervalMs idle/busy/backoff', () => {
  assert.equal(adminOpPollIntervalMs({ failStreak: 0, busy: false }), ADMIN_OP_POLL_IDLE_MS)
  assert.equal(adminOpPollIntervalMs({ failStreak: 0, busy: true }), ADMIN_OP_POLL_BUSY_MS)
  assert.equal(adminOpPollIntervalMs({ failStreak: 1, busy: true }), ADMIN_OP_POLL_FAIL_BACKOFF_MS[0])
  assert.equal(adminOpPollIntervalMs({ failStreak: 2, busy: false }), ADMIN_OP_POLL_FAIL_BACKOFF_MS[1])
  assert.equal(adminOpPollIntervalMs({ failStreak: 9, busy: false }), ADMIN_OP_POLL_FAIL_BACKOFF_MS[3])
})

test('nextAdminOpFailStreak', () => {
  assert.equal(nextAdminOpFailStreak(0, true), 0)
  assert.equal(nextAdminOpFailStreak(3, true), 0)
  assert.equal(nextAdminOpFailStreak(0, false), 1)
  assert.equal(nextAdminOpFailStreak(3, false), 4)
  assert.equal(nextAdminOpFailStreak(4, false), 4)
})

test('resolveAdminBusyNotify', () => {
  assert.deepEqual(resolveAdminBusyNotify(undefined, false), { kind: 'plain' })
  assert.deepEqual(resolveAdminBusyNotify(undefined, true), { kind: 'admin_busy', op: '' })
  assert.deepEqual(
    resolveAdminBusyNotify(
      {
        code: 'resource_exhausted',
        status: 429,
        message: 'admin heavy operation already in progress: flush',
        adminOpBusy: true,
        op: 'flush',
      },
      false,
    ),
    { kind: 'admin_busy', op: 'flush' },
  )
  assert.deepEqual(
    resolveAdminBusyNotify({ code: 'resource_exhausted', message: 'rate limit' }, false),
    { kind: 'plain' },
  )
})

test('commandAdminOpRefreshFeedback', () => {
  assert.deepEqual(commandAdminOpRefreshFeedback({ isAdmin: false }), { kind: 'denied' })
  assert.deepEqual(commandAdminOpRefreshFeedback({ isAdmin: true }), { kind: 'ok' })
  const errBusy = commandAdminOpRefreshFeedback({
    isAdmin: true,
    error: {
      code: 'resource_exhausted',
      status: 429,
      message: 'admin heavy operation already in progress: flush',
      adminOpBusy: true,
      op: 'flush',
    },
  })
  assert.equal(errBusy.kind, 'error')
  if (errBusy.kind === 'error') assert.equal(errBusy.adminBusy, true)
  const errPlain = commandAdminOpRefreshFeedback({
    isAdmin: true,
    errorMessage: 'network down',
  })
  assert.equal(errPlain.kind, 'error')
  if (errPlain.kind === 'error') {
    assert.equal(errPlain.adminBusy, false)
    assert.equal(errPlain.message, 'network down')
  }
})


test('shouldShowAdminOpLastBanner and dismiss storage', () => {
  assert.equal(
    shouldShowAdminOpLastBanner({
      isAdmin: true,
      busy: false,
      offline: false,
      lastSummary: 'Flush · ok · 12ms',
      lastFinishedAtUnix: 100,
      dismissedFinishedAtUnix: null,
    }),
    true,
  )
  assert.equal(
    shouldShowAdminOpLastBanner({
      isAdmin: true,
      busy: false,
      offline: false,
      lastSummary: 'Flush · ok',
      lastFinishedAtUnix: 100,
      dismissedFinishedAtUnix: 100,
    }),
    false,
  )
  const mem = new Map<string, string>()
  const storage = {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => {
      mem.set(k, v)
    },
  }
  assert.equal(writeDismissedAdminOpLastFinishedAt(storage, 42), true)
  assert.equal(readDismissedAdminOpLastFinishedAt(storage), 42)
})

test('adminOpLastToneClass', () => {
  assert.match(adminOpLastToneClass(true), /emerald/)
  assert.match(adminOpLastToneClass(false), /red/)
  assert.match(adminOpLastToneClass(null), /mts-muted|muted/)
})

test('adminOpLast banner/chip surface classes', () => {
  assert.match(adminOpLastBannerSurfaceClass(false), /border-red/)
  assert.match(adminOpLastBannerSurfaceClass(true), /emerald/)
  assert.match(adminOpLastBannerSurfaceClass(null), /slate/)
  assert.match(adminOpLastChipSurfaceClass(false), /red/)
  assert.match(adminOpLastChipSurfaceClass(true), /emerald/)
})

test('commandAdminOpLastDismissFeedback', () => {
  assert.deepEqual(commandAdminOpLastDismissFeedback({ isAdmin: false, hasLastSummary: true, alreadyDismissed: false }), {
    kind: 'denied',
  })
  assert.deepEqual(commandAdminOpLastDismissFeedback({ isAdmin: true, hasLastSummary: false, alreadyDismissed: false }), {
    kind: 'empty',
  })
  assert.deepEqual(commandAdminOpLastDismissFeedback({ isAdmin: true, hasLastSummary: true, alreadyDismissed: true }), {
    kind: 'already_dismissed',
  })
  assert.deepEqual(commandAdminOpLastDismissFeedback({ isAdmin: true, hasLastSummary: true, alreadyDismissed: false }), {
    kind: 'dismissed',
  })
})
