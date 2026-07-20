import assert from 'node:assert/strict'
import test from 'node:test'
import { formatRPDuration, mapRPDurationError, parseRPDurationToNs } from './rpDuration.ts'

test('parseRPDurationToNs common units', () => {
  assert.equal(parseRPDurationToNs('1s'), 1e9)
  assert.equal(parseRPDurationToNs('1h'), 3600e9)
  assert.equal(parseRPDurationToNs('7d'), 7 * 86400e9)
})

test('parseRPDurationToNs rejects bad', () => {
  assert.throws(() => parseRPDurationToNs('x'), /bad duration/)
})

test('formatRPDuration', () => {
  assert.equal(formatRPDuration(0), '0')
  assert.equal(formatRPDuration(3600e9), '1h')
})

test('mapRPDurationError', () => {
  const t = (k: string) => k
  try {
    parseRPDurationToNs('nope')
  } catch (e) {
    assert.equal(mapRPDurationError(e, t), 'databasesErrBadDuration')
  }
})
