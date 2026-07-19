import assert from 'node:assert/strict'
import test from 'node:test'
import { filterByName, filterDownsamplePolicies, filterUsers } from './listFilter.ts'

test('filterUsers by text and role', () => {
  const users = [
    { name: 'alice', display_name: 'Alice', role: 'admin' },
    { name: 'bob', display_name: 'Bobby', role: 'user' },
    { name: 'carol', display_name: '', role: 'user' },
  ]
  assert.equal(filterUsers(users, 'bo', '').length, 1)
  assert.equal(filterUsers(users, '', 'admin').length, 1)
  assert.equal(filterUsers(users, 'a', 'user').map((u) => u.name).join(','), 'carol')
})

test('filterByName', () => {
  const dbs = [{ name: 'metrics' }, { name: 'logs' }]
  assert.deepEqual(filterByName(dbs, 'log').map((d) => d.name), ['logs'])
  assert.equal(filterByName(dbs, '').length, 2)
})

test('filterDownsamplePolicies by text and enabled', () => {
  const policies = [
    {
      name: 'cpu_1m',
      source_database: 'metrics',
      source_measurement: 'cpu',
      target_database: 'metrics_ds',
      target_measurement: 'cpu_1m',
      enabled: true,
    },
    {
      name: 'mem_5m',
      source_database: 'metrics',
      source_measurement: 'mem',
      target_database: 'metrics_ds',
      target_measurement: 'mem_5m',
      enabled: false,
    },
  ]
  assert.equal(filterDownsamplePolicies(policies, 'cpu', '').length, 1)
  assert.equal(filterDownsamplePolicies(policies, '', 'enabled').map((p) => p.name).join(','), 'cpu_1m')
  assert.equal(filterDownsamplePolicies(policies, 'metrics', 'disabled').map((p) => p.name).join(','), 'mem_5m')
  assert.equal(filterDownsamplePolicies(policies, 'nope', '').length, 0)
})
