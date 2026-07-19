import assert from 'node:assert/strict'
import test from 'node:test'
import {
  EDGE_HTTPS_ACCEPTANCE_STEPS,
  edgeHttpsProgress,
} from './edgeHttpsAcceptance.ts'

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
  assert.ok(p.requiredTotal >= 2)
  assert.equal(p.done, 2)
  assert.ok(p.total >= p.done)
})
