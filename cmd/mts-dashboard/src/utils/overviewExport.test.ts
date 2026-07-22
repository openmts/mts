import assert from 'node:assert/strict'
import test from 'node:test'
import { buildOverviewExport, formatOverviewExportPretty } from './overviewExport.ts'

test('buildOverviewExport', () => {
  const out = buildOverviewExport(
    {
      connectivity: 'ok',
      healthy: true,
      ready: true,
      readiness_total: 80,
      downsample_status_summary: { total: 2, error: 1, lagging: 1, max_lag_seconds: 9 },
    },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.overview.snapshot')
  assert.equal(out.version, 2)
  assert.equal(out.healthy, true)
  assert.equal(out.readiness_total, 80)
  assert.equal(out.downsample_status_summary?.total, 2)
  assert.equal(out.downsample_status_summary?.error, 1)
  assert.equal(out.downsample_status_summary?.max_lag_seconds, 9)
})

test('formatOverviewExportPretty', () => {
  const text = formatOverviewExportPretty({ connectivity: 'ok' })
  assert.match(text, /mts.overview.snapshot/)
})
