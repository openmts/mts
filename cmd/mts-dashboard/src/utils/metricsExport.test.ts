import assert from 'node:assert/strict'
import test from 'node:test'
import { metricsFamiliesToJSON, metricsRefreshIntervalsMs } from './metricsExport.ts'

test('metricsFamiliesToJSON', () => {
  const out = metricsFamiliesToJSON([
    { name: 'a', type: 'counter', help: 'h', samples: [{ labels: {}, value: 1 }] },
  ])
  assert.equal(out.kind, 'mts.metrics.families')
  assert.equal(out.count, 1)
  assert.equal(out.families[0].name, 'a')
})

test('metricsRefreshIntervalsMs includes off', () => {
  assert.deepEqual(metricsRefreshIntervalsMs(), [0, 15_000, 30_000, 60_000])
})
