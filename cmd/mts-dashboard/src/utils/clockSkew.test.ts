import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_CLOCK_SKEW_WARN_SEC,
  clockSkewView,
  computeClockSkewSeconds,
  shouldShowClockSkewBanner,
} from './clockSkew.ts'

test('computeClockSkewSeconds null without sample', () => {
  assert.equal(computeClockSkewSeconds(null, 1), null)
  assert.equal(computeClockSkewSeconds(1, null), null)
})

test('computeClockSkewSeconds client ahead is positive', () => {
  const server = 1_700_000_000
  const checked = (server + 12) * 1000
  assert.equal(computeClockSkewSeconds(server, checked), 12)
  assert.equal(computeClockSkewSeconds(server, (server - 5) * 1000), -5)
})

test('clockSkewView urgency threshold', () => {
  const server = 1_700_000_000
  const ok = clockSkewView(server, (server + 10) * 1000)
  assert.equal(ok.urgency, 'ok')
  assert.equal(ok.label, '+10s')
  assert.equal(shouldShowClockSkewBanner(ok), false)

  const warn = clockSkewView(server, (server + DEFAULT_CLOCK_SKEW_WARN_SEC) * 1000)
  assert.equal(warn.urgency, 'warn')
  assert.equal(warn.label, '+30s')
  assert.equal(shouldShowClockSkewBanner(warn), true)

  const unknown = clockSkewView(null, null)
  assert.equal(unknown.urgency, 'unknown')
  assert.equal(shouldShowClockSkewBanner(unknown), false)
})
