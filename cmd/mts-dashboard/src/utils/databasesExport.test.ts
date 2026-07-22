import assert from 'node:assert/strict'
import test from 'node:test'
import { buildDatabasesExport, databasesToCSV } from './databasesExport.ts'

const rows = [
  { name: 'db,1', measurement_count: 2, retention_policy_count: 1, loaded: true },
]

test('buildDatabasesExport', () => {
  const out = buildDatabasesExport(rows, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.kind, 'mts.databases.inventory')
  assert.equal(out.count, 1)
  assert.equal(out.databases[0].name, 'db,1')
})

test('databasesToCSV escapes', () => {
  const csv = databasesToCSV(rows)
  assert.match(csv, /^name,measurement_count,retention_policy_count,loaded\n/)
  assert.match(csv, /"db,1"/)
})

test('buildDatabasesExport includes list_path meta', () => {
  const out = buildDatabasesExport(rows, new Date('2026-07-20T00:00:00.000Z'), {
    list_path: '/api/v1/data/databases',
    source: 'data',
  })
  assert.equal(out.version, 2)
  assert.equal(out.list_path, '/api/v1/data/databases')
  assert.equal(out.source, 'data')
})
