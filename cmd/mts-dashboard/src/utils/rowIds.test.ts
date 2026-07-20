import assert from 'node:assert/strict'
import test from 'node:test'
import { auditRowId, grantRowId } from './rowIds.ts'

test('auditRowId includes index for duplicates', () => {
  const e = { time: 't', user_name: 'u', action: 'a' }
  assert.notEqual(auditRowId(e, 0), auditRowId(e, 1))
  assert.ok(auditRowId(e, 0).includes('u'))
})

test('grantRowId stable', () => {
  assert.equal(
    grantRowId({ user: 'a', database: 'db', permission: 'read' }),
    grantRowId({ user: 'a', database: 'db', permission: 'read' }),
  )
  assert.notEqual(
    grantRowId({ user: 'a', database: 'db', permission: 'read' }),
    grantRowId({ user: 'a', database: 'db', permission: 'write' }),
  )
})
