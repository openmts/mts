import assert from 'node:assert/strict'
import test from 'node:test'
import {
  effectiveSessionRemainingMs,
  formatRemaining,
  parseExpiresAt,
  projectServerRemainingMs,
  sessionExpiryView,
  sessionViewFromRemainingMs,
} from './sessionExpiry.ts'

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

test('projectServerRemainingMs decays from check time', () => {
  const checked = 1_000_000
  assert.equal(projectServerRemainingMs(120, checked, checked), 120_000)
  assert.equal(projectServerRemainingMs(120, checked, checked + 30_000), 90_000)
  assert.equal(projectServerRemainingMs(10, checked, checked + 20_000), 0)
  assert.equal(projectServerRemainingMs(null, checked, checked), null)
})

test('effectiveSessionRemainingMs takes min of local and server', () => {
  const checked = 1_000_000
  const now = checked + 10_000
  // local 5m, server projected 50s -> 50s
  assert.equal(effectiveSessionRemainingMs(5 * 60_000, 60, checked, now), 50_000)
  // server larger -> local wins
  assert.equal(effectiveSessionRemainingMs(30_000, 120, checked, now), 30_000)
})

test('sessionViewFromRemainingMs', () => {
  assert.equal(sessionViewFromRemainingMs(0, true).urgency, 'expired')
  assert.equal(sessionViewFromRemainingMs(60_000, true).urgency, 'critical')
  assert.equal(sessionViewFromRemainingMs(5 * 60_000, true).urgency, 'warn')
  assert.equal(sessionViewFromRemainingMs(30 * 60_000, true).urgency, 'ok')
  assert.equal(sessionViewFromRemainingMs(0, false).urgency, 'unknown')
})
