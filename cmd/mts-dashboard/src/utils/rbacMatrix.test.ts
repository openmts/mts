import assert from 'node:assert/strict'
import test from 'node:test'
import {
  RBAC_CAPABILITY_MATRIX,
  capabilitiesForRole,
  countByLevel,
  levelForRole,
  matrixAreas,
} from './rbacMatrix.ts'

test('matrix covers core commercial surfaces', () => {
  const ids = RBAC_CAPABILITY_MATRIX.map((r) => r.id)
  for (const need of ['query', 'write', 'operations', 'databases', 'users-grant', 'storage']) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(matrixAreas().length >= 4)
})

test('admin has full access on management capabilities', () => {
  const ops = RBAC_CAPABILITY_MATRIX.find((r) => r.id === 'operations')
  assert.ok(ops)
  assert.equal(levelForRole(ops, 'admin'), 'full')
  assert.equal(levelForRole(ops, 'user'), 'none')
})

test('user retains data-scoped query/write', () => {
  const q = RBAC_CAPABILITY_MATRIX.find((r) => r.id === 'query')
  const w = RBAC_CAPABILITY_MATRIX.find((r) => r.id === 'write')
  assert.equal(levelForRole(q!, 'user'), 'data_scoped')
  assert.equal(levelForRole(w!, 'user'), 'data_scoped')
  const userCaps = capabilitiesForRole('user')
  assert.ok(userCaps.some((r) => r.id === 'query'))
  assert.ok(!userCaps.some((r) => r.id === 'operations'))
})

test('countByLevel totals match matrix size', () => {
  const a = countByLevel('admin')
  const u = countByLevel('user')
  const totalA = a.full + a.self + a.data_scoped + a.none
  const totalU = u.full + u.self + u.data_scoped + u.none
  assert.equal(totalA, RBAC_CAPABILITY_MATRIX.length)
  assert.equal(totalU, RBAC_CAPABILITY_MATRIX.length)
  assert.ok(a.full > u.full)
  assert.ok(u.none > a.none)
})
