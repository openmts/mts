import assert from 'node:assert/strict'
import test from 'node:test'
import { activeQueryApiPath, alignQueryContract } from './queryContractAlign.ts'
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
      { id: 'query_result_meta', path: '/api/v1/data/query/rows', enabled: true, description: '' },
      { id: 'query_stats_path', path: '/api/v1/data/query/stats', enabled: true, description: '' },
      { id: 'query_stream_end_meta', path: '/api/v1/data/query/stream', enabled: true, description: '' },
      { id: 'delete_response_meta', path: '/api/v1/data/delete', enabled: true, description: '' },
      { id: 'data_limits', path: '/api/v1/data/limits', enabled: true, description: '' },
      { id: 'meta_list_path', path: '/api/v1/meta/databases', enabled: true, description: '' },
    ],
    enabledCount: 6,
    totalFeatures: 6,
    missingRequired: [],
    complete: true,
    ...partial,
  }
}

test('activeQueryApiPath maps modes', () => {
  assert.equal(activeQueryApiPath('rows'), '/api/v1/data/query/rows')
  assert.equal(activeQueryApiPath('columns'), '/api/v1/data/query/columns')
  assert.equal(activeQueryApiPath('explain'), '/api/v1/data/query/explain')
  assert.equal(activeQueryApiPath('stream-row'), '/api/v1/data/query/stream')
  assert.equal(activeQueryApiPath('stream-column'), '/api/v1/data/query/stream')
})

test('alignQueryContract ok', () => {
  const a = alignQueryContract({
    contract: contract(),
    limits: { maxWritePoints: 1, defaultQueryLimit: 1000, maxQueryLimit: 100000, path: '/api/v1/data/limits' },
    queryMode: 'columns',
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.query_features_ok, true)
  assert.equal(a.limits_match_contract, true)
  assert.equal(a.active_query_path, '/api/v1/data/query/columns')
  assert.equal(a.recommend_columns, false)
})

test('alignQueryContract warns limits mismatch and recommends columns for rows', () => {
  const a = alignQueryContract({
    contract: contract({ maxQueryLimit: 5000 }),
    limits: { maxWritePoints: 1, defaultQueryLimit: 1, maxQueryLimit: 100000, path: '/api/v1/data/limits' },
    queryMode: 'rows',
  })
  assert.equal(a.limits_match_contract, false)
  assert.equal(a.recommend_columns, true)
  assert.equal(a.tone, 'warn')
})

test('alignQueryContract bad missing features', () => {
  const a = alignQueryContract({
    contract: contract({
      features: [{ id: 'data_limits', path: '/api/v1/data/limits', enabled: true, description: '' }],
      enabledCount: 1,
      totalFeatures: 1,
    }),
    queryMode: 'rows',
  })
  assert.equal(a.query_features_ok, false)
  assert.ok(a.missing_query_features.includes('query_result_meta'))
  assert.equal(a.tone, 'bad')
})
