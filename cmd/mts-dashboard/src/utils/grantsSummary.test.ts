import assert from 'node:assert/strict'
import test from 'node:test'
import { filterGrantRows, flattenUserGrants, grantCoverage } from './grantsSummary.ts'

test('flattenUserGrants sorts by user/db/perm', () => {
  const rows = flattenUserGrants([
    {
      user: 'bob',
      role: 'user',
      grants: [
        { database: 'metrics', permission: 'read' },
        { database: 'default', permission: 'write' },
      ],
    },
    {
      user: 'alice',
      role: 'user',
      grants: [{ database: 'default', permission: 'read' }],
    },
  ])
  assert.deepEqual(
    rows.map((r) => `${r.user}:${r.database}:${r.permission}`),
    ['alice:default:read', 'bob:default:write', 'bob:metrics:read'],
  )
})

test('filter and coverage', () => {
  const rows = flattenUserGrants([
    { user: 'a', grants: [{ database: 'd1', permission: 'read' }, { database: 'd2', permission: 'write' }] },
    { user: 'b', grants: [{ database: 'd1', permission: 'admin' }] },
  ])
  assert.equal(filterGrantRows(rows, { database: 'd1' }).length, 2)
  assert.equal(filterGrantRows(rows, { q: 'admin' }).length, 1)
  const cov = grantCoverage(rows)
  assert.equal(cov.users, 2)
  assert.equal(cov.databases, 2)
  assert.equal(cov.grants, 3)
})
