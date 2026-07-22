import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clampQueryLimitInput,
  normalizeDataLimits,
  queryLimitExceedsMax,
  writePointsExceedsMax,
} from './dataLimitsView.ts'

test('normalizeDataLimits', () => {
  const v = normalizeDataLimits({
    max_write_points: 10,
    default_query_limit: 20,
    max_query_limit: 30,
    path: '/api/v1/data/limits',
  })
  assert.equal(v.maxWritePoints, 10)
  assert.equal(v.maxQueryLimit, 30)
})

test('exceeds and clamp', () => {
  assert.equal(queryLimitExceedsMax(1001, 1000), true)
  assert.equal(queryLimitExceedsMax(1000, 1000), false)
  assert.equal(writePointsExceedsMax(11, 10), true)
  assert.equal(clampQueryLimitInput(5000, 1000), 1000)
  assert.equal(clampQueryLimitInput(50, 1000), 50)
})
