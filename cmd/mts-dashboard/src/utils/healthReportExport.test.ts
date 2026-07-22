import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildHealthReport,
  formatHealthReportPretty,
  healthReportFilename,
  HEALTH_REPORT_KIND,
} from './healthReportExport.ts'

test('buildHealthReport aggregates overview and downsample', () => {
  const out = buildHealthReport(
    {
      connectivity: 'ok',
      healthy: true,
      ready: true,
      readiness_total: 70,
      downsample_status_summary: { total: 3, error: 1, lagging: 1, max_lag_seconds: 5 },
      ops_stats: { compaction_active: 0 },
    },
    new Date('2026-07-22T00:00:00.000Z'),
  )
  assert.equal(out.kind, HEALTH_REPORT_KIND)
  assert.equal(out.version, 1)
  assert.equal(out.overview.version, 2)
  assert.equal(out.downsample_status_summary?.error, 1)
  assert.equal(out.downsample_tone, 'bad')
  assert.equal((out.ops_stats as { compaction_active: number }).compaction_active, 0)
  assert.match(out.disclaimer, /健康报告/)
})

test('format and filename', () => {
  const text = formatHealthReportPretty({ connectivity: 'ok' })
  assert.match(text, /mts.health.report/)
  assert.match(healthReportFilename(new Date('2026-07-22T01:02:03.000Z')), /mts-health-report-/)
})
