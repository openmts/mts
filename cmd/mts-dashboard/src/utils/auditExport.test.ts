import assert from 'node:assert/strict'
import test from 'node:test'
import { auditEventsToCSV } from './auditExport.ts'

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
