import assert from 'node:assert/strict'
import test from 'node:test'
import {
  PRODUCTION_CHECKLIST,
  automatedCoverage,
  productionChecklistJump,
  requiredChecklist,
} from './productionChecklist.ts'
import { textForLocale } from './localizedText.ts'

test('production checklist has required commercial gates', () => {
  const ids = PRODUCTION_CHECKLIST.map((x) => x.id)
  for (const need of [
    'https-edge',
    'security-headers',
    'change-default-admin',
    'smoke-login-query-write',
    'admin-op-visibility',
    'user-disable-revokes-tokens',
    'batch-admin-last',
    'downsample-advanced-form',
    'downsample-policy-detail',
    'downsample-policy-deep-link',
    'downsample-status-health',
    'readiness-downsample-health-card',
    'ops-downsample-health-card',
    'password-policy-public',
    'session-remaining-calibration',
    'login-session-seed',
    'session-sample-source',
    'overview-session-server-hint',
    'clock-skew-banner',
    'api-spec-password-policy',
    'api-spec-auth-session-seed',
    'error-codes-remediation',
    'write-accepted-points',
    'query-result-meta',
    'query-result-path-visible',
    'query-result-scope',
    'query-result-export-meta',
    'query-stats-path',
    'delete-result-export-meta',
    'meta-list-path',
    'databases-meta-path',
    'data-limits-endpoint',
    'stream-delete-meta',
    'data-contract-endpoint',
    'overview-data-contract',
    'overview-data-contract-jump',
    'acceptance-data-contract',
    'write-empty-aligned',
    'write-response-mode',
    'write-result-export-meta',
    'write-response-retention',
    'databases-meas-path',
    'ops-maintenance-path',
    'admin-config-storage-path',
    'users-doctor-path',
    'meta-downsample-path',
    'storage-auth-path',
    'session-policy-audit-path',
    'ops-config-stats-path',
    'storage-overview-path',
    'readiness-storage-result-path',
    'storage-validate-metrics-path',
    'storage-export-write-path',
    'config-effective-summary',
    'metrics-health-signals',
    'readiness-storage-drill-handoff',
    'write-contract-align',
    'query-contract-align',
    'ops-action-summary',
    'databases-meta-align',
    'audit-session-summary',
    'users-meta-align',
    'access-grants-meta-align',
    'access-matrix-meta-align',
  ]) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(requiredChecklist().length >= 4)
})

test('automated coverage is partial but non-zero', () => {
  const cov = automatedCoverage()
  assert.ok(cov.total >= 5)
  assert.ok(cov.automated >= 2)
  assert.ok(cov.ratio > 0 && cov.ratio < 1)
})

test('productionChecklist bilingual titles and details', () => {
  for (const item of PRODUCTION_CHECKLIST) {
    assert.ok(item.title.zh && item.title.en, item.id + ' title')
    assert.ok(item.detail.zh && item.detail.en, item.id + ' detail')
    assert.ok(textForLocale(item.title, 'en'))
    assert.ok(textForLocale(item.detail, 'zh'))
  }
})

test('admin-op-visibility is automated commercial gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'admin-op-visibility')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.equal(item?.severity, 'recommended')
})

test('production checklist items expose jump targets', () => {
  for (const item of PRODUCTION_CHECKLIST) {
    const jump = productionChecklistJump(item)
    assert.ok(jump, item.id)
    assert.ok(jump.startsWith('/') || jump.startsWith('#'), item.id + ' jump shape')
  }
  const adminOp = PRODUCTION_CHECKLIST.find((x) => x.id === 'admin-op-visibility')
  assert.equal(productionChecklistJump(adminOp!), '/operations#ops-status-strip')
  const disableRevoke = PRODUCTION_CHECKLIST.find((x) => x.id === 'user-disable-revokes-tokens')
  assert.equal(productionChecklistJump(disableRevoke!), '/users?status=disabled#users-filter-bar')
  assert.equal(productionChecklistJump({}), null)
})

test('batch-admin-last is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'batch-admin-last')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.equal(item?.severity, 'recommended')
  assert.equal(productionChecklistJump(item!), '/users#users-filter-bar')
})

test('downsample-advanced-form is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'downsample-advanced-form')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.equal(item?.severity, 'recommended')
  assert.ok(productionChecklistJump(item!)?.includes('/downsample'))
})

test('downsample-policy-detail is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'downsample-policy-detail')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/downsample'))
})



test('downsample-policy-deep-link is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'downsample-policy-deep-link')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('policy='))
  assert.ok(productionChecklistJump(item!)?.includes('downsample-detail'))
})


test('downsample-status-health is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'downsample-status-health')
  assert.ok(item)
  assert.equal(item?.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('downsample-status'))
  assert.ok(productionChecklistJump(item!)?.includes('health=error'))
})

test('readiness-downsample-health-card is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'readiness-downsample-health-card')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('downsample-health-panel'))
})

test('ops-downsample-health-card is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'ops-downsample-health-card')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/operations'))
})

test('password policy and session calibration jumps', () => {
  const policy = PRODUCTION_CHECKLIST.find((x) => x.id === 'password-policy-public')
  assert.ok(productionChecklistJump(policy!)?.includes('account-password-policy'))
  const session = PRODUCTION_CHECKLIST.find((x) => x.id === 'session-remaining-calibration')
  assert.ok(productionChecklistJump(session!)?.includes('account-session'))
  const spec = PRODUCTION_CHECKLIST.find((x) => x.id === 'api-spec-password-policy')
  assert.ok(productionChecklistJump(spec!)?.includes('password-policy'))
  assert.ok(requiredChecklist().some((x) => x.id === 'password-policy-public'))
  assert.ok(requiredChecklist().some((x) => x.id === 'session-remaining-calibration'))
})

test('overview-session-server-hint is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'overview-session-server-hint')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('overview-summary'))
})

test('clock-skew-banner is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'clock-skew-banner')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('account-session'))
})

test('login-session-seed is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'login-session-seed')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('account-session'))
  assert.ok(requiredChecklist().some((x) => x.id === 'login-session-seed'))
})

test('session-sample-source is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'session-sample-source')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
})

test('api-spec-auth-session-seed is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'api-spec-auth-session-seed')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('remaining_seconds'))
})


test('error-codes-remediation is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'error-codes-remediation')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('error-codes'))
  assert.ok(requiredChecklist().some((x) => x.id === 'error-codes-remediation'))
})


test('write-accepted-points is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'write-accepted-points')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/write'))
  assert.ok(requiredChecklist().some((x) => x.id === 'write-accepted-points'))
})


test('query-result-meta is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'query-result-meta')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/query'))
  assert.ok(requiredChecklist().some((x) => x.id === 'query-result-meta'))
})


test('query-result-path-visible is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'query-result-path-visible')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('query-results'))
})


test('query-stats-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'query-stats-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('query-stats'))
})


test('delete-result-export-meta is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'delete-result-export-meta')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/query'))
})


test('data-limits-endpoint is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'data-limits-endpoint')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/write'))
  assert.ok(requiredChecklist().some((x) => x.id === 'data-limits-endpoint'))
})


test('stream-delete-meta is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'stream-delete-meta')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/query'))
  assert.ok(requiredChecklist().some((x) => x.id === 'stream-delete-meta'))
})


test('data-contract-endpoint is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'data-contract-endpoint')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('commercial-handoff'))
  assert.ok(requiredChecklist().some((x) => x.id === 'data-contract-endpoint'))
})


test('overview-data-contract is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'overview-data-contract')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('overview-summary'))
})


test('overview-data-contract-jump is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'overview-data-contract-jump')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('overview-summary'))
})


test('acceptance-data-contract is automated required gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'acceptance-data-contract')
  assert.ok(item)
  assert.equal(item!.severity, 'required')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('commercial-handoff'))
  assert.ok(requiredChecklist().some((x) => x.id === 'acceptance-data-contract'))
})


test('meta-list-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'meta-list-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/query'))
})


test('databases-meta-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'databases-meta-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/databases'))
})


test('write-response-retention is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'write-response-retention')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/write'))
})


test('databases-meas-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'databases-meas-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/databases'))
})


test('ops-maintenance-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'ops-maintenance-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/ops'))
})


test('admin-config-storage-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'admin-config-storage-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/config'))
})

test('users-doctor-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'users-doctor-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/users'))
})

test('meta-downsample-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'meta-downsample-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/databases'))
})

test('storage-auth-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'storage-auth-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/storage'))
})

test('session-policy-audit-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'session-policy-audit-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/account'))
})

test('ops-config-stats-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'ops-config-stats-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/operations'))
})

test('storage-overview-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'storage-overview-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/storage'))
})

test('readiness-storage-result-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'readiness-storage-result-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/storage'))
})

test('storage-validate-metrics-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'storage-validate-metrics-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/storage'))
})

test('storage-export-write-path is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'storage-export-write-path')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/storage'))
})

test('config-effective-summary is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'config-effective-summary')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/config'))
})

test('metrics-health-signals is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'metrics-health-signals')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/observability/metrics'))
})

test('readiness-storage-drill-handoff is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'readiness-storage-drill-handoff')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/ops/readiness'))
})

test('write-contract-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'write-contract-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/write'))
})

test('query-contract-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'query-contract-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/query'))
})

test('ops-action-summary is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'ops-action-summary')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/operations'))
})

test('databases-meta-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'databases-meta-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.ok(productionChecklistJump(item!)?.includes('/databases'))
})

test('audit-session-summary is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'audit-session-summary')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.equal(item!.jump, '/audit')
})

test('users-meta-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'users-meta-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.equal(item!.jump, '/users')
})

test('access-grants-meta-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'access-grants-meta-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.equal(item!.jump, '/access/grants')
})

test('access-matrix-meta-align is automated recommended gate', () => {
  const item = PRODUCTION_CHECKLIST.find((x) => x.id === 'access-matrix-meta-align')
  assert.ok(item)
  assert.equal(item!.severity, 'recommended')
  assert.equal(item!.automated, true)
  assert.equal(item!.jump, '/access')
})
