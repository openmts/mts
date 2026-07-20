import assert from 'node:assert/strict'
import test from 'node:test'
import { classifyLatency, latencyFromNanos, latencyLevelLabel, nanosToMs } from './queryLatency.ts'

test('nanosToMs converts', () => {
  assert.equal(nanosToMs(1_000_000), 1)
  assert.equal(nanosToMs(-1), 0)
})

test('classifyLatency levels and locale labels', () => {
  assert.equal(classifyLatency(10).level, 'fast')
  assert.equal(classifyLatency(10).label, '快速')
  assert.equal(classifyLatency(10, undefined, 'en').label, 'Fast')
  assert.equal(classifyLatency(100).level, 'ok')
  assert.equal(classifyLatency(100, undefined, 'en').label, 'Normal')
  assert.equal(classifyLatency(500).level, 'slow')
  assert.equal(classifyLatency(500, undefined, 'en').label, 'Slow')
  assert.equal(classifyLatency(2000).level, 'critical')
  assert.equal(classifyLatency(2000, undefined, 'en').label, 'Very slow')
  assert.ok(classifyLatency(2000).barPercent <= 100)
})

test('latencyLevelLabel', () => {
  assert.equal(latencyLevelLabel('fast', 'zh'), '快速')
  assert.equal(latencyLevelLabel('critical', 'en'), 'Very slow')
})

test('latencyFromNanos', () => {
  assert.equal(latencyFromNanos(80_000_000).level, 'ok')
  assert.equal(latencyFromNanos(80_000_000, undefined, 'en').label, 'Normal')
})
