import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildDownsampleStatusSummaryExport,
  downsampleStatusSummaryJump,
  downsampleStatusSummaryToCSV,
  downsampleStatusSummaryTone,
  downsampleStatusesToCSV,
  normalizeDownsampleStatusSummary,
  summarizeDownsampleStatuses,
} from './downsampleStatusSummary.ts'

test('normalizeDownsampleStatusSummary', () => {
  const s = normalizeDownsampleStatusSummary({ total: 3, enabled: 2, error: 1, lagging: 1, max_lag_seconds: 12 })
  assert.equal(s.total, 3)
  assert.equal(s.error, 1)
  assert.equal(normalizeDownsampleStatusSummary(null).total, 0)
})

test('summarizeDownsampleStatuses local fallback', () => {
  const s = summarizeDownsampleStatuses([
    { policy_name: 'a', enabled: true, active: true, lag_seconds: 0 },
    { policy_name: 'b', enabled: true, last_error: 'x', lag_seconds: 9 },
  ])
  assert.equal(s.total, 2)
  assert.equal(s.enabled, 2)
  assert.equal(s.active, 1)
  assert.equal(s.error, 1)
  assert.equal(s.lagging, 1)
  assert.equal(s.max_lag_seconds, 9)
})

test('tone and jump', () => {
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ error: 1 })), 'bad')
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ lagging: 1 })), 'warn')
  assert.equal(downsampleStatusSummaryTone(normalizeDownsampleStatusSummary({ total: 1 })), 'ok')
  assert.match(downsampleStatusSummaryJump(normalizeDownsampleStatusSummary({ error: 1 })), /downsample-status/)
})

test('export summary json/csv', () => {
  const s = normalizeDownsampleStatusSummary({ total: 2, enabled: 1, error: 1, lagging: 1, max_lag_seconds: 3 })
  const exp = buildDownsampleStatusSummaryExport(s, {
    at: new Date('2026-07-22T00:00:00.000Z'),
    filter_health: 'error',
    statuses: [{ policy_name: 'p1', last_error: 'boom', lag_seconds: 3 }],
  })
  assert.equal(exp.kind, 'mts.downsample.status_summary')
  assert.equal(exp.version, 1)
  assert.equal(exp.filter_health, 'error')
  assert.equal(exp.statuses?.[0]?.policy_name, 'p1')
  const csv = downsampleStatusSummaryToCSV(s)
  assert.match(csv, /^total,enabled/)
  assert.match(csv, /2,1,0,1,1,3/)
  const rows = downsampleStatusesToCSV([{ policy_name: 'p1', last_error: 'e,x', lag_seconds: 1 }])
  assert.match(rows, /policy_name/)
  assert.match(rows, /"e,x"/)
})
