import assert from 'node:assert/strict'
import test from 'node:test'
import { buildAuditSessionSummary, preferredAuditListPath } from './auditSessionSummary.ts'

test('preferredAuditListPath admin/self', () => {
  assert.equal(preferredAuditListPath({ source: 'admin' }), '/api/v1/admin/audit')
  assert.equal(
    preferredAuditListPath({ source: 'self', userName: 'alice' }),
    '/api/v1/users/alice/audit',
  )
})

test('buildAuditSessionSummary ok with path', () => {
  const s = buildAuditSessionSummary({
    listPath: '/api/v1/admin/audit',
    source: 'admin',
    eventCount: 10,
    filteredCount: 8,
    serverTotal: 10,
    limit: 500,
  })
  assert.equal(s.tone, 'ok')
  assert.equal(s.path_ok, true)
  assert.equal(s.event_count, 10)
  assert.equal(s.filtered_count, 8)
  assert.equal(s.list_path, '/api/v1/admin/audit')
})

test('buildAuditSessionSummary warn when truncated vs total', () => {
  const s = buildAuditSessionSummary({
    listPath: '/api/v1/admin/audit',
    source: 'admin',
    eventCount: 50,
    filteredCount: 50,
    serverTotal: 200,
  })
  assert.equal(s.tone, 'warn')
  assert.equal(s.server_total, 200)
})

test('buildAuditSessionSummary self path', () => {
  const s = buildAuditSessionSummary({
    source: 'self',
    userName: 'bob',
    eventCount: 2,
    filteredCount: 2,
  })
  assert.equal(s.list_path, '/api/v1/users/bob/audit')
  assert.equal(s.source, 'self')
  assert.equal(s.tone, 'ok')
})
