import assert from 'node:assert/strict'
import test from 'node:test'
import { buildGrantsExport, grantsToCSV } from './grantsExport.ts'

const rows = [
  { user: 'a', role: 'user', disabled: false, database: 'db,1', permission: 'read' },
]

test('buildGrantsExport', () => {
  const out = buildGrantsExport(rows, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.access.grants')
  assert.equal(out.count, 1)
})

test('grantsToCSV escapes', () => {
  const csv = grantsToCSV(rows)
  assert.match(csv, /^user,role,disabled,database,permission\n/)
  assert.match(csv, /"db,1"/)
})
