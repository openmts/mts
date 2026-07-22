import assert from 'node:assert/strict'
import test from 'node:test'
import { buildExportPreflight } from './exportPreflight.ts'
import { buildOpsNextSteps } from './opsNextSteps.ts'

test('buildOpsNextSteps prioritizes signoff and checklist warns', () => {
  const preflight = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 0.2,
    edgeHttpsRequiredRatio: 0.5,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: false,
    signoffNotes: { edgeHttps: 'x' },
    deployKitReviewed: false,
  })
  const steps = buildOpsNextSteps({ locale: 'zh', preflight, signoffNotes: { edgeHttps: 'x' }, limit: 3 })
  assert.ok(steps.length >= 1)
  assert.equal(steps[0]?.id, 'signoff')
  assert.match(steps[0]?.message ?? '', /签核备注/)
  assert.ok(steps.some((s) => s.id === 'checklist'))
  assert.ok(steps.every((s) => s.id !== 'footer'))
})

test('buildOpsNextSteps returns done when all ok', () => {
  const preflight = buildExportPreflight({
    locale: 'en',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    deployKitReviewed: true,
  })
  const steps = buildOpsNextSteps({
    locale: 'en',
    preflight,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
  })
  assert.equal(steps.length, 1)
  assert.equal(steps[0]?.id, 'done')
  assert.match(steps[0]?.message ?? '', /deployment-side/i)
})

test('buildOpsNextSteps respects limit and deploy kit info', () => {
  const preflight = buildExportPreflight({
    locale: 'en',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    deployKitReviewed: false,
  })
  const steps = buildOpsNextSteps({
    locale: 'en',
    preflight,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    limit: 2,
  })
  assert.ok(steps.some((s) => s.id === 'deployKit'))
  assert.ok(steps.length <= 2)
})

test('buildOpsNextSteps prioritizes large clock skew', () => {
  const preflight = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    deployKitReviewed: true,
  })
  const steps = buildOpsNextSteps({
    locale: 'zh',
    preflight,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    clockSkewSeconds: 45,
    limit: 2,
  })
  assert.equal(steps[0]?.id, 'clockSkew')
  assert.match(steps[0]?.message ?? '', /45s/)
  assert.ok(steps[0]?.target?.includes('account-session'))
})
