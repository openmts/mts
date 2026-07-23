import assert from 'node:assert/strict'
import test from 'node:test'
import { buildUsersExport, usersToCSV } from './usersExport.ts'

const rows = [
  { name: 'alice', display_name: 'A,1', role: 'user', disabled: false },
]

test('buildUsersExport v2 meta', () => {
  const out = buildUsersExport(rows, new Date('2026-07-20T00:00:00.000Z'), {
    list_path: '/api/v1/users',
    batch_path: '/api/v1/users/batch-disabled',
  })
  assert.equal(out.kind, 'mts.users.inventory')
  assert.equal(out.version, 2)
  assert.equal(out.count, 1)
  assert.equal(out.list_path, '/api/v1/users')
  assert.equal(out.batch_path, '/api/v1/users/batch-disabled')
})

test('usersToCSV escapes', () => {
  const csv = usersToCSV(rows)
  assert.match(csv, /^name,display_name,role,disabled\n/)
  assert.match(csv, /"A,1"/)
})
