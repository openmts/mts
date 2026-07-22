import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildDownsamplePolicyDetailFields,
  formatDownsampleFunctions,
  formatDownsamplePolicyPath,
} from './downsamplePolicyDetail.ts'

const sample = {
  name: 'p1',
  source_database: 'default',
  source_retention: 'autogen',
  source_measurement: 'cpu',
  target_database: 'default',
  target_retention: 'autogen',
  target_measurement: 'cpu_1m',
  interval: 60e9,
  refresh_interval: 60e9,
  lookback: 120e9,
  batch_size: 100,
  enabled: true,
  group_by_tags: ['host'],
  functions: [{ function: 'mean', field: 'usage', as: 'mean_usage' }],
}

test('formatDownsamplePolicyPath includes retention', () => {
  assert.match(formatDownsamplePolicyPath(sample), /default\/autogen\/cpu/)
  assert.match(formatDownsamplePolicyPath(sample), /cpu_1m/)
})

test('formatDownsampleFunctions', () => {
  assert.equal(formatDownsampleFunctions(sample.functions), 'mean(usage) as mean_usage')
  assert.equal(formatDownsampleFunctions([]), '')
})

test('buildDownsamplePolicyDetailFields', () => {
  const fields = buildDownsamplePolicyDetailFields(sample, (ns) => `${Number(ns) / 1e9}s`)
  assert.ok(fields.some((f) => f.key === 'refresh' && f.value === '60s'))
  assert.ok(fields.some((f) => f.key === 'lookback' && f.value === '120s'))
  assert.ok(fields.some((f) => f.key === 'batch_size' && f.value === '100'))
  assert.ok(fields.some((f) => f.key === 'functions' && f.value.includes('mean_usage')))
  assert.deepEqual(buildDownsamplePolicyDetailFields(null, () => ''), [])
})
