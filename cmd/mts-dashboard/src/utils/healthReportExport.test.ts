import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyServerPasswordPolicy,
  resetPasswordPolicyRuntime,
} from './passwordPolicy.ts'
import {
  buildHealthReport,
  formatCommercialHandoffClipboardText,
  formatHealthReportMarkdown,
  formatHealthReportPretty,
  healthReportFilename,
  healthReportFilenames,
  HEALTH_REPORT_KIND,
  HEALTH_REPORT_VERSION,
} from './healthReportExport.ts'

test('buildHealthReport aggregates overview downsample and handoff', () => {
  resetPasswordPolicyRuntime()
  applyServerPasswordPolicy({ min_length: 8, version: 2, max_auth_ttl_seconds: 3600, default_auth_ttl_seconds: 1800 })
  const now = Date.parse('2026-07-22T00:00:00.000Z')
  const out = buildHealthReport(
    {
      connectivity: 'ok',
      healthy: true,
      ready: true,
      readiness_total: 70,
      downsample_status_summary: { total: 3, error: 1, lagging: 1, max_lag_seconds: 5 },
      ops_stats: { compaction_active: 0 },
      session_expires_at: new Date(now + 600_000).toISOString(),
      session_remaining_seconds: 90,
      session_checked_at_ms: now,
      session_server_time_unix: Math.floor(now / 1000) - 3,
    },
    new Date(now),
  )
  assert.equal(out.kind, HEALTH_REPORT_KIND)
  assert.equal(out.version, HEALTH_REPORT_VERSION)
  assert.equal(out.version, 2)
  assert.equal(out.overview.version, 2)
  assert.equal(out.downsample_status_summary?.error, 1)
  assert.equal(out.downsample_tone, 'bad')
  assert.equal((out.ops_stats as { compaction_active: number }).compaction_active, 0)
  assert.equal(out.commercial_handoff.session_calibration.calibration_source, 'merged')
  assert.equal(out.commercial_handoff.session_calibration.clock_skew_seconds, 3)
  assert.equal(out.commercial_handoff.password_policy.server_synced, true)
  assert.match(out.disclaimer, /健康报告/)
  resetPasswordPolicyRuntime()
})

test('format and filename include commercial handoff', () => {
  resetPasswordPolicyRuntime()
  const text = formatHealthReportPretty({ connectivity: 'ok' })
  assert.match(text, /mts.health.report/)
  assert.match(text, /commercial_handoff/)
  assert.match(healthReportFilename(new Date('2026-07-22T01:02:03.000Z')), /mts-health-report-/)
  const md = formatHealthReportMarkdown({
    connectivity: 'ok',
    healthy: true,
    downsample_status_summary: { total: 1, error: 1, lagging: 0, max_lag_seconds: 0 },
  }, new Date('2026-07-22T00:00:00.000Z'))
  assert.match(md, /# MTS health report/)
  assert.match(md, /Downsample/)
  assert.match(md, /error: 1/)
  assert.match(md, /Commercial handoff/)
  assert.match(md, /password_policy/)
  const names = healthReportFilenames(new Date('2026-07-22T01:02:03.000Z'))
  assert.match(names.json, /\.json$/)
  assert.match(names.md, /\.md$/)
  const clip = formatCommercialHandoffClipboardText(
    buildHealthReport({ connectivity: 'ok' }).commercial_handoff,
    new Date('2026-07-22T00:00:00.000Z'),
  )
  assert.match(clip, /MTS commercial handoff/)
  assert.match(clip, /password_policy/)
})
