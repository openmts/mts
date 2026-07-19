import assert from 'node:assert/strict'
import test from 'node:test'
import { filterByName, filterUsers } from './listFilter.ts'

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
