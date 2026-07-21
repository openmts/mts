import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  defaultApiTimeoutHintMs,
  elapsedSince,
  formatElapsedSeconds,
  isLongRunning,
} from './inFlightStatus.ts'

test('formatElapsedSeconds', () => {
  assert.equal(formatElapsedSeconds(0), '0s')
  assert.equal(formatElapsedSeconds(4500), '4s')
  assert.equal(formatElapsedSeconds(65_000), '1m05s')
})

test('isLongRunning', () => {
  assert.equal(isLongRunning(4_999), false)
  assert.equal(isLongRunning(5_000), true)
})

test('elapsedSince clamps', () => {
  assert.equal(elapsedSince(null, 1000), 0)
  assert.equal(elapsedSince(800, 1000), 200)
})

test('defaultApiTimeoutHintMs', () => {
  assert.equal(defaultApiTimeoutHintMs(null), 30_000)
  assert.equal(defaultApiTimeoutHintMs(60_000), 60_000)
})
