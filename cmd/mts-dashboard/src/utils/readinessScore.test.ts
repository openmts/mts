import assert from 'node:assert/strict'
import test from 'node:test'
import {
  computeReadinessScore,
  doctorScorePart,
  readinessLevel,
} from './readinessScore.ts'

test('doctorScorePart penalizes warns and missing tls', () => {
  const full = doctorScorePart({
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
  })
  assert.equal(full.score, 1)

  const warns = doctorScorePart({
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 2,
    httpTlsEnabled: true,
  })
  assert.ok(warns.score < 1)
  assert.ok(warns.reasons.some((r) => r.startsWith('doctor_warns')))

  const noTls = doctorScorePart({
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: false,
  })
  assert.ok(noTls.score < 1)
  assert.ok(noTls.reasons.includes('http_tls_disabled'))

  const missing = doctorScorePart({ doctorLoaded: false })
  assert.equal(missing.score, 0.4)
  assert.ok(missing.reasons.includes('doctor_unavailable'))
})

test('computeReadinessScore averages four dimensions', () => {
  const perfect = computeReadinessScore({
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
  })
  assert.equal(perfect.total, 100)
  assert.equal(readinessLevel(perfect.total), 'good')

  const partial = computeReadinessScore({
    requiredChecklistRatio: 0.5,
    edgeHttpsRequiredRatio: 0.5,
    backupScheduleRequiredRatio: 0.5,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 2,
    httpTlsEnabled: false,
  })
  assert.ok(partial.total < 80)
  assert.ok(partial.reasons.includes('checklist_incomplete'))
  assert.ok(partial.reasons.includes('http_tls_disabled'))
  assert.notEqual(readinessLevel(partial.total), 'good')
})
