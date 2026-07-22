import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildMaintenanceErrorsExport,
  buildOpsStatsExport,
  formatOpsStatsPretty,
  maintenanceErrorsToText,
} from './opsExport.ts'

test('buildMaintenanceErrorsExport', () => {
  const out = buildMaintenanceErrorsExport(['a', ''], new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.ops.maintenance_errors')
  assert.equal(out.count, 1)
})

test('maintenanceErrorsToText', () => {
  assert.equal(maintenanceErrorsToText(['x', 'y']), 'x\ny')
})

test('buildOpsStatsExport', () => {
  const out = buildOpsStatsExport(
    {
      maintenance: { compaction_active: 1 },
      compaction: { total: 2 },
      memory: { heap_alloc_bytes: 3 },
      maintenance_errors: ['e1'],
      downsample_status_summary: { total: 4, error: 1, lagging: 2, max_lag_seconds: 12 },
    },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.ops.stats')
  assert.equal(out.version, 2)
  assert.equal(out.maintenance_error_count, 1)
  assert.equal((out.compaction as { total: number }).total, 2)
  assert.equal((out.memory as { heap_alloc_bytes: number }).heap_alloc_bytes, 3)
  assert.equal(out.downsample_status_summary?.error, 1)
  assert.equal(out.downsample_status_summary?.max_lag_seconds, 12)
})

test('formatOpsStatsPretty', () => {
  const text = formatOpsStatsPretty({ maintenance: { a: 1 } })
  assert.match(text, /mts.ops.stats/)
})
