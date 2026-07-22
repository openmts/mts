import assert from 'node:assert/strict'
import test from 'node:test'
import { alignDatabasesMeta, preferredDatabasesListPath } from './databasesMetaAlign.ts'

test('preferredDatabasesListPath', () => {
  assert.equal(preferredDatabasesListPath('admin'), '/api/v1/admin/databases')
  assert.equal(preferredDatabasesListPath('data'), '/api/v1/data/databases')
  assert.equal(preferredDatabasesListPath(''), '/api/v1/data/databases')
})

test('alignDatabasesMeta data source ok', () => {
  const a = alignDatabasesMeta({
    listPath: '/api/v1/data/databases',
    source: 'data',
    databaseCount: 3,
    loadedDetailCount: 1,
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.source_ok, true)
  assert.equal(a.database_count, 3)
  assert.equal(a.loaded_detail_count, 1)
  assert.equal(a.list_path, '/api/v1/data/databases')
})

test('alignDatabasesMeta manual is bad', () => {
  const a = alignDatabasesMeta({ source: 'manual', databaseCount: 0 })
  assert.equal(a.tone, 'bad')
  assert.equal(a.source_ok, false)
  assert.equal(a.preferred_list_path, '/api/v1/data/databases')
})

test('alignDatabasesMeta partial is warn', () => {
  const a = alignDatabasesMeta({ source: 'partial', listPath: '/api/v1/data/databases', databaseCount: 2 })
  assert.equal(a.tone, 'warn')
  assert.equal(a.source_ok, true)
})
