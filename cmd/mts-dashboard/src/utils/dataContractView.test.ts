import assert from 'node:assert/strict'
import test from 'node:test'
import {
  assertAcceptanceDataContractShape,
  buildDataContractView,
  formatDataContractHandoffLine,
  requiredDataContractFeatureIds,
  toAcceptanceDataContractSummary,
} from './dataContractView.ts'

test('buildDataContractView empty is incomplete', () => {
  const v = buildDataContractView(null)
  assert.equal(v.loaded, false)
  assert.equal(v.complete, false)
  assert.deepEqual(v.missingRequired, [...requiredDataContractFeatureIds()])
})

test('buildDataContractView complete when required features enabled', () => {
  const features = requiredDataContractFeatureIds().map((id) => ({
    id,
    path: `/p/${id}`,
    enabled: true,
  }))
  const v = buildDataContractView({
    version: 1,
    path: '/api/v1/data/contract',
    max_write_points: 1000,
    default_query_limit: 100,
    max_query_limit: 5000,
    features,
  })
  assert.equal(v.loaded, true)
  assert.equal(v.complete, true)
  assert.equal(v.enabledCount, features.length)
  assert.match(formatDataContractHandoffLine(v), /complete/)
  assert.match(formatDataContractHandoffLine(v), /max_write=1000/)
})

test('missing feature keeps incomplete', () => {
  const features = requiredDataContractFeatureIds()
    .slice(0, 3)
    .map((id) => ({ id, enabled: true }))
  const v = buildDataContractView({ features })
  assert.equal(v.complete, false)
  assert.ok(v.missingRequired.includes('delete_response_meta'))
})


test('toAcceptanceDataContractSummary and shape guard', () => {
  const s = toAcceptanceDataContractSummary({
    version: 1,
    path: '/api/v1/data/contract',
    max_write_points: 1000,
    features: requiredDataContractFeatureIds().map((id) => ({ id, enabled: true })),
  })
  assert.equal(s.loaded, true)
  assert.equal(s.complete, true)
  assert.match(s.summary_line, /complete/)
  assert.equal(assertAcceptanceDataContractShape(s), true)
  assert.equal(assertAcceptanceDataContractShape({}), false)
})
