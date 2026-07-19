import test from 'node:test'
import assert from 'node:assert/strict'
import { parseHumanDurationToNs, formatNsDuration } from './duration.ts'

test('parseHumanDurationToNs supports common units', () => {
  assert.equal(parseHumanDurationToNs('1s'), 1e9)
  assert.equal(parseHumanDurationToNs('1m'), 60e9)
  assert.equal(parseHumanDurationToNs('1h'), 3600e9)
  assert.equal(parseHumanDurationToNs('1d'), 86400e9)
})

test('parseHumanDurationToNs rejects invalid', () => {
  assert.throws(() => parseHumanDurationToNs(''))
  assert.throws(() => parseHumanDurationToNs('abc'))
  assert.throws(() => parseHumanDurationToNs('0m'))
})

test('formatNsDuration roundtrip-ish', () => {
  assert.equal(formatNsDuration(60e9), '1m')
  assert.equal(formatNsDuration(1e9), '1s')
})
