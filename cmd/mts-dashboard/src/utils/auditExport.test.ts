import assert from 'node:assert/strict'
import test from 'node:test'
import { auditEventsToCSV, buildAuditExport } from './auditExport.ts'

test('auditEventsToCSV header and escape', () => {
  const csv = auditEventsToCSV([
    {
      time: 't1',
      user_name: 'alice',
      action: 'flush',
      database: 'db,1',
      detail: 'say "hi"',
    },
  ])
  assert.match(csv, /^time,user_name,action,database,detail\n/)
  assert.match(csv, /"db,1"/)
  assert.match(csv, /"say ""hi"""/)
})

test('buildAuditExport v2 meta', () => {
  const out = buildAuditExport(
    [{ time: 't1', user_name: 'a', action: 'login' }],
    new Date('2026-07-23T00:00:00.000Z'),
    {
      list_path: '/api/v1/admin/audit',
      source: 'admin',
      server_total: 9,
      filtered_count: 1,
      limit: 500,
    },
  )
  assert.equal(out.kind, 'mts.audit.export')
  assert.equal(out.version, 2)
  assert.equal(out.count, 1)
  assert.equal(out.list_path, '/api/v1/admin/audit')
  assert.equal(out.source, 'admin')
  assert.equal(out.server_total, 9)
})
