import assert from 'node:assert/strict'
import test from 'node:test'
import { auditLimitOptions, buildAuditQueryString } from './auditQuery.ts'

test('buildAuditQueryString includes defaults and filters', () => {
  const qs = buildAuditQueryString({
    userName: 'alice',
    action: 'flush',
    sinceUnix: 10,
    untilUnix: 20,
    limit: 100,
  })
  const p = new URLSearchParams(qs)
  assert.equal(p.get('user_name'), 'alice')
  assert.equal(p.get('action'), 'flush')
  assert.equal(p.get('since_unix'), '10')
  assert.equal(p.get('until_unix'), '20')
  assert.equal(p.get('limit'), '100')
})

test('buildAuditQueryString default limit 500', () => {
  const p = new URLSearchParams(buildAuditQueryString({}))
  assert.equal(p.get('limit'), '500')
})

test('auditLimitOptions', () => {
  assert.deepEqual(auditLimitOptions(), [100, 250, 500, 1000])
})
