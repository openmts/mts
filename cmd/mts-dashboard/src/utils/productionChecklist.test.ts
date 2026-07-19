import assert from 'node:assert/strict'
import test from 'node:test'
import {
  PRODUCTION_CHECKLIST,
  automatedCoverage,
  requiredChecklist,
} from './productionChecklist.ts'

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
