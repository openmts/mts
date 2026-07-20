import assert from 'node:assert/strict'
import test from 'node:test'
import { buildUsersExport, usersToCSV } from './usersExport.ts'

const rows = [
  { name: 'alice', display_name: 'A,1', role: 'user', disabled: false },
]

test('buildUsersExport', () => {
  const out = buildUsersExport(rows, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.users.inventory')
  assert.equal(out.count, 1)
})

test('usersToCSV escapes', () => {
  const csv = usersToCSV(rows)
  assert.match(csv, /^name,display_name,role,disabled\n/)
  assert.match(csv, /"A,1"/)
})
