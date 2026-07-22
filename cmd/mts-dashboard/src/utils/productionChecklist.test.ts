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
    'overview-session-server-hint',
    'clock-skew-banner',
    'api-spec-password-policy',
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
