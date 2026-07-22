import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyServerPasswordPolicy,
  resetPasswordPolicyRuntime,
} from './passwordPolicy.ts'
import {
  buildCommercialHandoffSummary,
  formatDataContractHandoffLine,
  buildPasswordPolicyHandoffSummary,
  buildSessionCalibrationHandoffSummary,
  formatPasswordPolicyHandoffLine,
  formatSessionCalibrationHandoffLine,
} from './commercialHandoffSummary.ts'

test('password policy handoff reflects runtime', () => {
  resetPasswordPolicyRuntime()
  const local = buildPasswordPolicyHandoffSummary()
  assert.equal(local.server_synced, false)
  assert.equal(local.min_length, 8)
  applyServerPasswordPolicy({
    min_length: 10,
    forbidden_defaults: ['admin'],
    default_auth_ttl_seconds: 7200,
    max_auth_ttl_seconds: 86400,
    version: 2,
  })
  const synced = buildPasswordPolicyHandoffSummary()
  assert.equal(synced.server_synced, true)
  assert.equal(synced.min_length, 10)
  assert.equal(synced.default_auth_ttl_seconds, 7200)
  assert.match(formatPasswordPolicyHandoffLine(synced), /min_length: 10/)
  resetPasswordPolicyRuntime()
})

test('session calibration handoff merges server remaining', () => {
  const now = Date.parse('2026-07-22T12:00:00.000Z')
  const expires = new Date(now + 10 * 60_000).toISOString()
  const merged = buildSessionCalibrationHandoffSummary({
    expiresAtIso: expires,
    serverRemainingSec: 90,
    checkedAtMs: now,
    nowMs: now + 30_000,
  })
  assert.equal(merged.calibration_source, 'session')
  assert.equal(merged.sample_source, 'session')
  const fromLogin = buildSessionCalibrationHandoffSummary({
    expiresAtIso: expires,
    serverRemainingSec: 90,
    checkedAtMs: now,
    sampleSource: 'login',
    nowMs: now + 30_000,
  })
  assert.equal(fromLogin.calibration_source, 'login')
  assert.equal(fromLogin.sample_source, 'login')
  assert.match(formatSessionCalibrationHandoffLine(fromLogin), /sample: login/)
  assert.equal(merged.calibrated_remaining_seconds, 60)
  assert.equal(merged.urgency, 'critical')
  assert.match(formatSessionCalibrationHandoffLine(merged), /source: session/)

  const localOnly = buildSessionCalibrationHandoffSummary({
    expiresAtIso: expires,
    nowMs: now,
  })
  assert.equal(localOnly.calibration_source, 'local_only')
  // 10 分钟 remaining 处于 warn 阈值（默认 10m）边界，urgency 为 warn
  assert.equal(localOnly.urgency, 'warn')
  assert.equal(localOnly.calibrated_remaining_seconds, 600)
  assert.equal(localOnly.clock_skew_seconds, null)

  const withSkew = buildSessionCalibrationHandoffSummary({
    expiresAtIso: expires,
    serverRemainingSec: 90,
    checkedAtMs: now + 5_000,
    serverTimeUnix: Math.floor(now / 1000),
    nowMs: now + 5_000,
  })
  assert.equal(withSkew.clock_skew_seconds, 5)
  assert.equal(withSkew.server_time_unix, Math.floor(now / 1000))
  assert.match(formatSessionCalibrationHandoffLine(withSkew), /skew: \+5s/)
})

test('buildCommercialHandoffSummary bundles both', () => {
  resetPasswordPolicyRuntime()
  const s = buildCommercialHandoffSummary({ expiresAtIso: null })
  assert.equal(s.session_calibration.has_local_expiry, false)
  assert.equal(s.password_policy.min_length, 8)
})


test('commercial handoff includes data_contract view', () => {
  const h = buildCommercialHandoffSummary({
    dataContract: {
      version: 1,
      path: '/api/v1/data/contract',
      max_write_points: 10,
      features: [
        { id: 'write_accepted_points', enabled: true },
        { id: 'write_response_mode', enabled: true },
        { id: 'query_result_meta', enabled: true },
        { id: 'query_stream_end_meta', enabled: true },
        { id: 'delete_response_meta', enabled: true },
        { id: 'data_limits', enabled: true },
      ],
    },
  })
  assert.equal(h.data_contract.complete, true)
  assert.match(formatDataContractHandoffLine(h.data_contract), /complete/)
})
