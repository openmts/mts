import assert from 'node:assert/strict'
import test from 'node:test'
import {
  classifyReachability,
  nextFailStreak,
  probeOutcomeFromStatus,
  shouldShowAfterFailures,
} from './serverReachability.ts'

test('classifyReachability offline wins', () => {
  const v = classifyReachability('offline', 'fail')
  assert.equal(v.kind, 'offline')
  assert.equal(v.showUnreachableBanner, false)
})

test('classifyReachability ok and unreachable', () => {
  assert.equal(classifyReachability('online', 'ok').kind, 'ok')
  assert.equal(classifyReachability('online', 'ok').showUnreachableBanner, false)
  const u = classifyReachability('online', 'fail')
  assert.equal(u.kind, 'unreachable')
  assert.equal(u.showUnreachableBanner, true)
  assert.equal(classifyReachability('online', 'skipped').kind, 'unknown')
})

test('fail streak and threshold', () => {
  assert.equal(nextFailStreak(0, false), 1)
  assert.equal(nextFailStreak(1, false), 2)
  assert.equal(nextFailStreak(3, true), 0)
  assert.equal(shouldShowAfterFailures(1, 2), false)
  assert.equal(shouldShowAfterFailures(2, 2), true)
})

test('probeOutcomeFromStatus', () => {
  assert.equal(probeOutcomeFromStatus(200), 'ok')
  assert.equal(probeOutcomeFromStatus(204), 'ok')
  assert.equal(probeOutcomeFromStatus(503), 'fail')
  assert.equal(probeOutcomeFromStatus(null), 'fail')
})
