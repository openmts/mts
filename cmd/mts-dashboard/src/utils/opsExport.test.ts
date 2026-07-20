import assert from 'node:assert/strict'
import test from 'node:test'
import { buildMaintenanceErrorsExport, maintenanceErrorsToText } from './opsExport.ts'

test('buildMaintenanceErrorsExport', () => {
  const out = buildMaintenanceErrorsExport(['a', '', 'b'], new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.ops.maintenance_errors')
  assert.equal(out.count, 2)
  assert.deepEqual(out.errors, ['a', 'b'])
})

test('maintenanceErrorsToText', () => {
  assert.equal(maintenanceErrorsToText(['x', 'y']), 'x\ny')
  assert.equal(maintenanceErrorsToText(null), '')
})
