import assert from 'node:assert/strict'
import test from 'node:test'
import { buildExportPreflight } from './exportPreflight.ts'

test('buildExportPreflight reports gaps and completeness', () => {
  const gaps = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 0.5,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 0,
    doctorLoaded: false,
    signoffNotes: { edgeHttps: 'x' },
    deployKitReviewed: false,
  })
  assert.equal(gaps.readyToExport, true)
  assert.ok(gaps.warnCount >= 3)
  assert.ok(gaps.items.some((i) => i.id === 'checklist' && i.level === 'warn'))
  assert.ok(gaps.items.some((i) => i.id === 'signoff' && /签核备注未齐/.test(i.message)))
  assert.ok(gaps.items.some((i) => i.id === 'footer' && i.level === 'info'))

  const full = buildExportPreflight({
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
  assert.equal(full.warnCount, 0)
  assert.ok(full.okCount >= 5)
  assert.ok(full.items.some((i) => i.id === 'signoff' && /All three/.test(i.message)))
})

test('doctor warns and tls info', () => {
  const r = buildExportPreflight({
    locale: 'en',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: false,
    doctorWarnCount: 2,
    httpTlsEnabled: false,
    signoffNotes: {
      edgeHttps: 'a',
      backupOffsite: 'b',
      backupAlert: 'c',
    },
    deployKitReviewed: true,
  })
  assert.ok(r.items.some((i) => i.id === 'doctor-ok' && i.level === 'warn'))
  assert.ok(r.items.some((i) => i.id === 'doctor-warn' && /2/.test(i.message)))
  assert.ok(r.items.some((i) => i.id === 'tls' && i.level === 'info'))
})
