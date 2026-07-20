import assert from 'node:assert/strict'
import test from 'node:test'
import { filterDatabaseNames, grantKey, sortGrants } from './grantPanelView.ts'

test('filterDatabaseNames', () => {
  assert.deepEqual(filterDatabaseNames(['metrics', 'logs', 'sys'], 'me'), ['metrics'])
  assert.equal(filterDatabaseNames(['a'], '').length, 1)
})

test('sortGrants and grantKey', () => {
  const sorted = sortGrants([
    { database: 'b', permission: 'write' },
    { database: 'a', permission: 'read' },
    { database: 'a', permission: 'admin' },
  ])
  assert.equal(sorted[0].database, 'a')
  assert.equal(sorted[0].permission, 'admin')
  assert.equal(grantKey('db', 'read'), 'db\u0000read')
})
