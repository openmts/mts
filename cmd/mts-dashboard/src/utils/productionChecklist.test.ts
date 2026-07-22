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
  for (const need of ['https-edge', 'security-headers', 'change-default-admin', 'smoke-login-query-write', 'admin-op-visibility']) {
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
  assert.equal(productionChecklistJump({}), null)
})

