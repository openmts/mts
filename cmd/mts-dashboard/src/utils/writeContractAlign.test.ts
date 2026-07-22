import assert from 'node:assert/strict'
import test from 'node:test'
import { alignWriteContract, preferredWriteApiPath } from './writeContractAlign.ts'
import type { DataContractView } from './dataContractView.ts'

function contract(partial: Partial<DataContractView> = {}): DataContractView {
  return {
    loaded: true,
    path: '/api/v1/data/contract',
    version: 1,
    maxWritePoints: 10000,
    defaultQueryLimit: 1000,
    maxQueryLimit: 100000,
    features: [
      { id: 'write_accepted_points', path: '/api/v1/data/write', enabled: true, description: '' },
      { id: 'write_response_mode', path: '/api/v1/data/write', enabled: true, description: '' },
      { id: 'write_response_retention', path: '/api/v1/data/write', enabled: true, description: '' },
      { id: 'data_limits', path: '/api/v1/data/limits', enabled: true, description: '' },
    ],
    enabledCount: 4,
    totalFeatures: 4,
    missingRequired: [],
    complete: true,
    ...partial,
  }
}

test('preferredWriteApiPath', () => {
  assert.equal(preferredWriteApiPath('typed', false), '/api/v1/data/write/typed')
  assert.equal(preferredWriteApiPath('form', true), '/api/v1/data/write/points-typed')
  assert.equal(preferredWriteApiPath('line', false), '/api/v1/data/write')
})

test('alignWriteContract ok typed', () => {
  const a = alignWriteContract({
    contract: contract(),
    limits: { maxWritePoints: 10000, defaultQueryLimit: 1000, maxQueryLimit: 100000, path: '/api/v1/data/limits' },
    writeMode: 'typed',
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.write_features_ok, true)
  assert.equal(a.limits_match_contract, true)
  assert.equal(a.preferred_write_path, '/api/v1/data/write/typed')
  assert.equal(a.recommend_typed, false)
})

test('alignWriteContract warns non-typed and limits mismatch', () => {
  const a = alignWriteContract({
    contract: contract({ maxWritePoints: 5000 }),
    limits: { maxWritePoints: 10000, defaultQueryLimit: 1, maxQueryLimit: 1, path: '/api/v1/data/limits' },
    writeMode: 'line',
  })
  assert.equal(a.limits_match_contract, false)
  assert.equal(a.recommend_typed, true)
  assert.equal(a.tone, 'warn')
})

test('alignWriteContract bad when write features missing', () => {
  const a = alignWriteContract({
    contract: contract({
      features: [{ id: 'data_limits', path: '/api/v1/data/limits', enabled: true, description: '' }],
      enabledCount: 1,
      totalFeatures: 1,
    }),
    writeMode: 'typed',
  })
  assert.equal(a.write_features_ok, false)
  assert.ok(a.missing_write_features.includes('write_accepted_points'))
  assert.equal(a.tone, 'bad')
})
