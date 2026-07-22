import assert from 'node:assert/strict'
import test from 'node:test'
import { buildExportPreflight, formatExportPreflightText, preflightItemTarget } from './exportPreflight.ts'

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

test('preflightItemTarget maps anchors', () => {
  assert.equal(preflightItemTarget('checklist')?.target, '#production-checklist')
  assert.equal(preflightItemTarget('signoff')?.target, '#signoff-notes')
  assert.equal(preflightItemTarget('deployKit')?.target, '#deploy-kit')
  assert.equal(preflightItemTarget('doctor-warn')?.target, '#doctor-panel')
  assert.equal(preflightItemTarget('footer'), null)
})

test('buildExportPreflight attaches jump targets', () => {
  const r = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 0,
    edgeHttpsRequiredRatio: 0,
    backupScheduleRequiredRatio: 0,
    doctorLoaded: false,
    signoffNotes: {},
    deployKitReviewed: false,
  })
  const signoff = r.items.find((i) => i.id === 'signoff')
  assert.equal(signoff?.target, '#signoff-notes')
  assert.equal(signoff?.actionKey, 'preflightJumpLocal')
  const footer = r.items.find((i) => i.id === 'footer')
  assert.equal(footer?.target, undefined)
})

test('formatExportPreflightText includes levels and disclaimer', () => {
  const r = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 0.5,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    signoffNotes: {},
    deployKitReviewed: true,
  })
  const text = formatExportPreflightText(r, 'zh')
  assert.match(text, /导出前预检/)
  assert.match(text, /\[warn\]/)
  assert.match(text, /不代表生产验收完成/)
  const en = formatExportPreflightText(r, 'en')
  assert.match(en, /export preflight/i)
})

test('buildExportPreflight admin op busy info', () => {
  const busy = buildExportPreflight({
    locale: 'zh',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    adminOpBusy: true,
    adminOpKindLabel: '刷盘 (flush)',
    signoffNotes: { edgeHttps: 'a', backupOffsite: 'b', backupAlert: 'c' },
    deployKitReviewed: true,
  })
  const busyItem = busy.items.find((i) => i.id === 'admin-op-busy')
  assert.ok(busyItem && busyItem.level === 'info')
  assert.match(busyItem!.message, /刷盘/)
  assert.equal(preflightItemTarget('admin-op-busy')?.target, '/operations#ops-status-strip')

  const idle = buildExportPreflight({
    locale: 'en',
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
    adminOpBusy: false,
    signoffNotes: { edgeHttps: 'a', backupOffsite: 'b', backupAlert: 'c' },
    deployKitReviewed: true,
  })
  assert.ok(idle.items.some((i) => i.id === 'admin-op-busy' && i.level === 'ok'))
})

test('buildExportPreflight clock skew warn and ok', () => {
  const warn = buildExportPreflight({
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
    clockSkewSeconds: 45,
  })
  const item = warn.items.find((i) => i.id === 'clockSkew')
  assert.ok(item)
  assert.equal(item?.level, 'warn')
  assert.match(item?.message ?? '', /45s/)
  assert.equal(item?.target, '/account#account-session')
  assert.ok(warn.warnCount >= 1)

  const ok = buildExportPreflight({
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
    clockSkewSeconds: 5,
  })
  assert.ok(ok.items.some((i) => i.id === 'clockSkew' && i.level === 'ok'))

  const unknown = buildExportPreflight({
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
    clockSkewSeconds: null,
  })
  assert.ok(unknown.items.some((i) => i.id === 'clockSkew' && i.level === 'info'))
})

test('preflightItemTarget clockSkew', () => {
  assert.equal(preflightItemTarget('clockSkew')?.target, '/account#account-session')
})


test('buildExportPreflight data contract complete/gap/missing', () => {
  const base = {
    locale: 'zh' as const,
    requiredChecklistRatio: 1,
    edgeHttpsRequiredRatio: 1,
    backupScheduleRequiredRatio: 1,
    doctorLoaded: true,
    doctorOk: true,
    doctorWarnCount: 0,
    httpTlsEnabled: true,
  }
  const ok = buildExportPreflight({ ...base, dataContractLoaded: true, dataContractComplete: true })
  assert.ok(ok.items.some((i) => i.id === 'data-contract' && i.level === 'ok'))
  const gap = buildExportPreflight({
    ...base,
    dataContractLoaded: true,
    dataContractComplete: false,
    dataContractMissing: ['delete_response_meta'],
  })
  const g = gap.items.find((i) => i.id === 'data-contract')
  assert.equal(g?.level, 'warn')
  assert.match(g?.message || '', /delete_response_meta/)
  const miss = buildExportPreflight({ ...base, dataContractLoaded: false })
  assert.ok(miss.items.some((i) => i.id === 'data-contract' && i.level === 'warn'))
  assert.equal(preflightItemTarget('data-contract')?.target, '/ops/readiness#commercial-handoff-panel')
})
