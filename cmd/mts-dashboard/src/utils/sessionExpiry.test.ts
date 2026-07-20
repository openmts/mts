import assert from 'node:assert/strict'
import test from 'node:test'
import { formatRemaining, parseExpiresAt, sessionExpiryView } from './sessionExpiry.ts'

test('parseExpiresAt', () => {
  assert.equal(parseExpiresAt(''), null)
  assert.equal(parseExpiresAt('not-a-date'), null)
  assert.ok(parseExpiresAt('2026-07-19T12:00:00.000Z')! > 0)
})

test('sessionExpiryView levels', () => {
  const now = 1_000_000
  assert.equal(sessionExpiryView(null, now).urgency, 'unknown')
  assert.equal(sessionExpiryView(now - 1, now).urgency, 'expired')
  assert.equal(sessionExpiryView(now - 1, now).label, '已过期')
  assert.equal(sessionExpiryView(now - 1, now, undefined, undefined, 'en').label, 'Expired')
  assert.equal(sessionExpiryView(now + 60_000, now).urgency, 'critical')
  assert.equal(sessionExpiryView(now + 5 * 60_000, now).urgency, 'warn')
  assert.equal(sessionExpiryView(now + 30 * 60_000, now).urgency, 'ok')
})

test('formatRemaining', () => {
  assert.equal(formatRemaining(0), '0m')
  assert.equal(formatRemaining(45_000), '45s')
  assert.equal(formatRemaining(125_000), '2m')
  assert.ok(formatRemaining(3_700_000).includes('h'))
})
