import assert from 'node:assert/strict'
import test from 'node:test'
import {
  downsampleStatusSummaryJump,
  downsampleStatusSummaryTone,
  normalizeDownsampleStatusSummary,
} from './downsampleStatusSummary.ts'

test('normalizeDownsampleStatusSummary', () => {
  const s = normalizeDownsampleStatusSummary({ total: 3, enabled: 2, error: 1, lagging: 1, max_lag_seconds: 12 })
  assert.equal(s.total, 3)
  assert.equal(s.error, 1)
  assert.equal(normalizeDownsampleStatusSummary(null).total, 0)
})

test('tone and jump', () => {
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ error: 1 })), 'bad')
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ lagging: 1 })), 'warn')
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ total: 1 })), 'ok')
  assert.match(downsampleStatusSummaryJump(normalizeDownsampleStatusSummary({ error: 1 })), /downsample-status/)
})
