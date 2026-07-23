import assert from 'node:assert/strict'
import test from 'node:test'
import { buildGrantsExport, grantsToCSV } from './grantsExport.ts'

const rows = [
  { user: 'a', role: 'user', disabled: false, database: 'db,1', permission: 'read' },
]

test('buildGrantsExport v2 meta', () => {
  const out = buildGrantsExport(rows, new Date('2026-07-20T00:00:00.000Z'), {
    users_list_path: '/api/v1/users',
    permissions_path_sample: '/api/v1/users/a/database-permissions',
    user_count: 1,
    database_count: 1,
  })
  assert.equal(out.kind, 'mts.access.grants')
  assert.equal(out.version, 2)
  assert.equal(out.count, 1)
  assert.equal(out.users_list_path, '/api/v1/users')
  assert.match(out.permissions_path_sample || '', /database-permissions/)
})

test('grantsToCSV escapes', () => {
  const csv = grantsToCSV(rows)
  assert.match(csv, /^user,role,disabled,database,permission\n/)
  assert.match(csv, /"db,1"/)
})
