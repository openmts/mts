import assert from 'node:assert/strict'
import test from 'node:test'
import {
  computeReadinessScore,
  doctorScorePart,
  formatReadinessReason,
  formatReadinessReasons,
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

test('computeReadinessScore flags admin_op_busy without tanking total alone', () => {
  const base = computeReadinessScore({
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    adminOpBusy: true,
  })
  assert.equal(base.total, 100)
  assert.ok(base.reasons.includes('admin_op_busy'))
  assert.equal(readinessLevel(base.total), 'good')
})

test('computeReadinessScore flags admin_op_last_failed and deducts lightly', () => {
  const ok = computeReadinessScore({
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
  })
  const failed = computeReadinessScore({
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    adminOpLastFailed: true,
  })
  assert.equal(ok.total, 100)
  assert.equal(failed.total, 95)
  assert.ok(failed.reasons.includes('admin_op_last_failed'))
  assert.equal(readinessLevel(failed.total), 'good')
})

test('formatReadinessReason localizes known codes', () => {
  assert.match(formatReadinessReason('admin_op_last_failed', 'zh'), /失败/)
  assert.match(formatReadinessReason('admin_op_busy', 'en'), /busy/i)
  assert.match(formatReadinessReason('doctor_warns:3', 'zh'), /3/)
  assert.match(formatReadinessReasons(['admin_op_busy', 'checklist_incomplete'], 'zh'), /；/)
})
