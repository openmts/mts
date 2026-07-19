import assert from 'node:assert/strict'
import test from 'node:test'
import { classifyLatency, latencyFromNanos, nanosToMs } from './queryLatency.ts'

test('nanosToMs converts', () => {
  assert.equal(nanosToMs(1_000_000), 1)
  assert.equal(nanosToMs(-1), 0)
})

test('classifyLatency levels', () => {
  assert.equal(classifyLatency(10).level, 'fast')
  assert.equal(classifyLatency(100).level, 'ok')
  assert.equal(classifyLatency(500).level, 'slow')
  assert.equal(classifyLatency(2000).level, 'critical')
  assert.ok(classifyLatency(2000).barPercent <= 100)
})

test('latencyFromNanos', () => {
  assert.equal(latencyFromNanos(80_000_000).level, 'ok')
})
