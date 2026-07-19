import assert from 'node:assert/strict'
import test from 'node:test'
import {
  EDGE_HTTPS_ACCEPTANCE_STEPS,
  edgeHttpsProgress,
} from './edgeHttpsAcceptance.ts'
import { textForLocale } from './localizedText.ts'

test('edge https acceptance has required gates', () => {
  const ids = EDGE_HTTPS_ACCEPTANCE_STEPS.map((s) => s.id)
  for (const need of ['tls-terminate', 'hsts-header', 'http-redirect', 'doctor-check']) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(EDGE_HTTPS_ACCEPTANCE_STEPS.some((s) => s.severity === 'required'))
})

test('edgeHttpsProgress counts required and overall', () => {
  const p = edgeHttpsProgress(['tls-terminate', 'hsts-header'])
  assert.equal(p.requiredDone, 2)
  assert.ok(p.requiredTotal >= 3)
  assert.ok(p.done >= 2)
})

test('edgeHttpsAcceptance bilingual titles and details', () => {
  for (const item of EDGE_HTTPS_ACCEPTANCE_STEPS) {
    assert.ok(item.title.zh && item.title.en, item.id + ' title')
    assert.ok(item.detail.zh && item.detail.en, item.id + ' detail')
    assert.ok(textForLocale(item.title, 'en'))
    assert.ok(textForLocale(item.detail, 'zh'))
  }
})
