import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sessionClockTickMs } from './sessionClock.ts'

test('sessionClockTickMs fine for critical/expired', () => {
  assert.equal(sessionClockTickMs('critical', 30_000), 1_000)
  assert.equal(sessionClockTickMs('expired', 0), 1_000)
})

test('sessionClockTickMs fine near warn end', () => {
  assert.equal(sessionClockTickMs('warn', 45_000), 1_000)
  assert.equal(sessionClockTickMs('warn', 90_000), 15_000)
})

test('sessionClockTickMs default for ok/unknown', () => {
  assert.equal(sessionClockTickMs('ok', 600_000), 15_000)
  assert.equal(sessionClockTickMs('unknown', 0), 15_000)
  assert.equal(sessionClockTickMs(null), 15_000)
})
