import assert from 'node:assert/strict'
import test from 'node:test'
import { buildDownsampleExport, downsampleToCSV } from './downsampleExport.ts'

const rows = [
  {
    name: 'p1',
    source_database: 'db,src',
    source_measurement: 'm',
    target_database: 'db',
    target_measurement: 'm_1h',
    interval: 3600,
    enabled: true,
  },
]

test('buildDownsampleExport', () => {
  const out = buildDownsampleExport(rows, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.downsample.policies')
  assert.equal(out.count, 1)
})

test('downsampleToCSV escapes', () => {
  const csv = downsampleToCSV(rows)
  assert.match(csv, /^name,source_database/)
  assert.match(csv, /"db,src"/)
})
