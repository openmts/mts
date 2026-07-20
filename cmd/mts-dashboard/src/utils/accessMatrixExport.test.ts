import assert from 'node:assert/strict'
import test from 'node:test'
import { accessMatrixToCSV, buildAccessMatrixExport } from './accessMatrixExport.ts'
import { RBAC_CAPABILITY_MATRIX } from './rbacMatrix.ts'

test('buildAccessMatrixExport localizes rows', () => {
  const out = buildAccessMatrixExport(RBAC_CAPABILITY_MATRIX.slice(0, 1), 'en', new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.access.matrix')
  assert.equal(out.count, 1)
  assert.equal(out.locale, 'en')
  assert.ok(out.rows[0].capability)
})

test('accessMatrixToCSV header and escape', () => {
  const csv = accessMatrixToCSV(RBAC_CAPABILITY_MATRIX.slice(0, 1), 'en')
  assert.ok(csv.startsWith('id,area,capability,admin,user,route,notes'))
  assert.ok(csv.includes(RBAC_CAPABILITY_MATRIX[0]!.id))
})
