import assert from 'node:assert/strict'
import test from 'node:test'
import { buildDownsampleExport, downsampleToCSV } from './downsampleExport.ts'

const rows = [
  {
    name: 'p1',
    source_database: 'db,src',
    source_measurement: 'm',
    source_retention: 'autogen',
    target_database: 'db',
    target_measurement: 'm_1h',
    target_retention: 'autogen',
    interval: 3600,
    refresh_interval: 3600,
    lookback: 3600,
    batch_size: 100,
    enabled: true,
    functions: [{ function: 'mean', field: 'usage', as: 'mean_usage' }],
    group_by_tags: ['host', 'region'],
  },
]

test('buildDownsampleExport v3 includes functions', () => {
  const out = buildDownsampleExport(rows, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.downsample.policies')
  assert.equal(out.version, 3)
  assert.equal(out.count, 1)
  assert.equal(out.policies[0].source_retention, 'autogen')
  assert.equal(out.policies[0].batch_size, 100)
  assert.equal(out.policies[0].functions_text, 'mean(usage) as mean_usage')
  assert.deepEqual(out.policies[0].group_by_tags, ['host', 'region'])
})

test('downsampleToCSV escapes and includes functions columns', () => {
  const csv = downsampleToCSV(rows)
  assert.match(csv, /^name,source_database/)
  assert.match(csv, /source_retention/)
  assert.match(csv, /refresh_interval/)
  assert.match(csv, /batch_size/)
  assert.match(csv, /functions/)
  assert.match(csv, /group_by_tags/)
  assert.match(csv, /"db,src"/)
  assert.match(csv, /mean\(usage\) as mean_usage/)
  assert.match(csv, /host\|region/)
})
