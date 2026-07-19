import assert from 'node:assert/strict'
import test from 'node:test'
import {
  RBAC_CAPABILITY_MATRIX,
  capabilitiesForRole,
  countByLevel,
  levelForRole,
  matrixAreas,
  textForLocale,
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

test('matrix rows expose bilingual labels', () => {
  const q = RBAC_CAPABILITY_MATRIX.find((r) => r.id === 'query')
  assert.ok(q)
  assert.equal(textForLocale(q.area, 'zh'), '数据面')
  assert.equal(textForLocale(q.area, 'en'), 'Data plane')
  assert.match(textForLocale(q.capability, 'en'), /Query/)
  assert.match(textForLocale(q.notes, 'en'), /read grant/)
  const areas = matrixAreas()
  assert.ok(areas.every((a) => a.key && a.label.zh && a.label.en))
})

test('every matrix row is bilingual', () => {
  for (const row of RBAC_CAPABILITY_MATRIX) {
    assert.ok(row.areaKey, row.id)
    assert.ok(row.area.zh && row.area.en, row.id + ' area')
    assert.ok(row.capability.zh && row.capability.en, row.id + ' capability')
    if (row.notes) {
      assert.ok(row.notes.zh && row.notes.en, row.id + ' notes')
    }
  }
  const areas = matrixAreas()
  assert.equal(new Set(areas.map((a) => a.key)).size, areas.length)
})
