import assert from 'node:assert/strict'
import test from 'node:test'
import {
  PRODUCTION_CHECKLIST,
  automatedCoverage,
  requiredChecklist,
} from './productionChecklist.ts'
import { textForLocale } from './localizedText.ts'

test('production checklist has required commercial gates', () => {
  const ids = PRODUCTION_CHECKLIST.map((x) => x.id)
  for (const need of ['https-edge', 'security-headers', 'change-default-admin', 'smoke-login-query-write']) {
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
