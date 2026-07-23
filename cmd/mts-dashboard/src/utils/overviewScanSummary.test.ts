import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildOverviewContractScan,
  buildOverviewDoctorScan,
  buildOverviewPathsScan,
  buildOverviewScanSummary,
} from './overviewScanSummary.ts'

test('buildOverviewDoctorScan counts levels', () => {
  const d = buildOverviewDoctorScan({
    path: '/api/v1/admin/doctor',
    checks: [{ level: 'ok' }, { level: 'warn' }, { level: 'error' }],
    httpTlsEnabled: true,
  })
  assert.equal(d.check_count, 3)
  assert.equal(d.warn_count, 1)
  assert.equal(d.error_count, 1)
  assert.equal(d.tone, 'bad')
  assert.equal(d.http_tls_enabled, true)
})

test('buildOverviewContractScan incomplete warn', () => {
  const c = buildOverviewContractScan({
    loaded: true,
    complete: false,
    enabledCount: 2,
    totalFeatures: 4,
    missingRequired: ['write'],
  })
  assert.equal(c.tone, 'warn')
  assert.deepEqual(c.missing_required, ['write'])
})

test('buildOverviewPathsScan ok', () => {
  const p = buildOverviewPathsScan({
    doctorPath: '/api/v1/admin/doctor',
    contractPath: '/api/v1/data/contract',
    healthPath: '/api/v1/admin/health',
  })
  assert.equal(p.tone, 'ok')
  assert.ok(p.path_count >= 5)
})

test('buildOverviewScanSummary merges tone', () => {
  const s = buildOverviewScanSummary({
    doctor: { checks: [{ level: 'warn' }] },
    contract: { loaded: true, complete: true, enabledCount: 3, totalFeatures: 3 },
  })
  assert.equal(s.tone, 'warn')
  assert.equal(s.doctor.warn_count, 1)
})
