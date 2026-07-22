import assert from 'node:assert/strict'
import test from 'node:test'
import { metricsFamiliesToJSON, metricsRefreshIntervalsMs } from './metricsExport.ts'

test('metricsFamiliesToJSON', () => {
  const out = metricsFamiliesToJSON(
    [{ name: 'a', type: 'counter', help: 'h', samples: [{ labels: {}, value: 1 }] }],
    { downsample_status_summary: { total: 2, error: 1, lagging: 1, max_lag_seconds: 9 } },
  )
  assert.equal(out.kind, 'mts.metrics.families')
  assert.equal(out.version, 2)
  assert.equal(out.count, 1)
  assert.equal(out.families[0].name, 'a')
  assert.equal(out.downsample_status_summary?.error, 1)
  assert.equal(out.downsample_status_summary?.max_lag_seconds, 9)
})

test('metricsFamiliesToJSON without summary', () => {
  const out = metricsFamiliesToJSON([])
  assert.equal(out.version, 2)
  assert.equal(out.downsample_status_summary, null)
})

test('metricsRefreshIntervalsMs includes off', () => {
  assert.deepEqual(metricsRefreshIntervalsMs(), [0, 15_000, 30_000, 60_000])
})
