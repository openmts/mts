import assert from 'node:assert/strict'
import test from 'node:test'
import { endpointMatchesQuery, filterApiSpecNamespaces } from './apiSpecFilter.ts'

const sample = [
  {
    name: 'health',
    endpoints: [
      { method: 'GET', path: '/healthz', description: 'liveness', response: 'healthResponse' },
    ],
  },
  {
    name: 'users',
    endpoints: [
      {
        method: 'POST',
        path: '/api/v1/users/batch-disabled',
        description: 'batch disable users',
        auth: 'admin',
        response: 'batchMutationResponse{ok,ok_count,admin_op_busy,last}',
      },
      {
        method: 'GET',
        path: '/api/v1/users',
        description: 'list users',
        auth: 'admin',
      },
    ],
  },
  {
    name: 'ops',
    endpoints: [
      { method: 'POST', path: '/api/v1/admin/flush', description: 'flush memtable', response: 'okResponse' },
    ],
  },
]

test('endpointMatchesQuery matches method path description response auth', () => {
  const ep = sample[1].endpoints[0]
  assert.equal(endpointMatchesQuery(ep, ''), true)
  assert.equal(endpointMatchesQuery(ep, 'batch-disabled'), true)
  assert.equal(endpointMatchesQuery(ep, 'batchMutation'), true)
  assert.equal(endpointMatchesQuery(ep, 'admin'), true)
  assert.equal(endpointMatchesQuery(ep, 'POST'), true)
  assert.equal(endpointMatchesQuery(ep, 'nope'), false)
})

test('filterApiSpecNamespaces scopes by ns without search', () => {
  const out = filterApiSpecNamespaces(sample, { ns: 'health' })
  assert.equal(out.length, 1)
  assert.equal(out[0].name, 'health')
  assert.equal(out[0].endpoints.length, 1)
})

test('filterApiSpecNamespaces filters within ns when hits exist', () => {
  const out = filterApiSpecNamespaces(sample, { ns: 'users', q: 'list' })
  assert.equal(out.length, 1)
  assert.equal(out[0].name, 'users')
  assert.equal(out[0].endpoints.length, 1)
  assert.match(out[0].endpoints[0].path, /\/users$/)
})

test('filterApiSpecNamespaces cross-ns fallback when current ns has no hits', () => {
  // default first ns is health; searching batch-disabled should expand to users
  const out = filterApiSpecNamespaces(sample, { ns: 'health', q: 'batch-disabled' })
  assert.ok(out.some((ns) => ns.name === 'users'))
  assert.ok(out.every((ns) => ns.endpoints.length > 0))
  const paths = out.flatMap((ns) => ns.endpoints.map((e) => e.path))
  assert.ok(paths.some((p) => p.includes('batch-disabled')))
  assert.ok(!paths.some((p) => p === '/healthz'))
})

test('filterApiSpecNamespaces empty q keeps empty-endpoint ns when no text filter', () => {
  const empty = [{ name: 'empty', endpoints: [] as { method: string; path: string }[] }]
  const out = filterApiSpecNamespaces(empty, { q: '' })
  assert.equal(out.length, 1)
  assert.equal(out[0].name, 'empty')
})

test('filterApiSpecNamespaces null-safe', () => {
  assert.deepEqual(filterApiSpecNamespaces(null, { q: 'x' }), [])
  assert.deepEqual(filterApiSpecNamespaces(undefined, {}), [])
})
