import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildAuditPrefillPath,
  buildQueryPrefillPath,
  buildWritePrefillPath,
  parseWritePrefill,
  queryFormToPrefill,
  isPrefillTimeRange,
  parseAuditPrefill,
  parseQueryPrefill,
  timeRangeToMsBounds,
  timeRangeToQueryFormTimes,
} from './routePrefill.ts'

test('isPrefillTimeRange', () => {
  assert.equal(isPrefillTimeRange('1h'), true)
  assert.equal(isPrefillTimeRange('24h'), true)
  assert.equal(isPrefillTimeRange('bad'), false)
  assert.equal(isPrefillTimeRange(1), false)
})

test('timeRangeToMsBounds and form times', () => {
  const now = 1_700_000_000_000
  const b = timeRangeToMsBounds('1h', now)
  assert.equal(b.endMs, now)
  assert.equal(b.startMs, now - 3600_000)
  const f = timeRangeToQueryFormTimes('24h', now)
  assert.equal(f.end_time, String(now))
  assert.equal(f.start_time, String(now - 24 * 3600_000))
})

test('parseQueryPrefill', () => {
  assert.deepEqual(parseQueryPrefill({ range: '1h', database: 'db1', measurement: 'cpu' }), {
    range: '1h',
    database: 'db1',
    measurement: 'cpu',
  })
  assert.deepEqual(parseQueryPrefill({ range: 'nope', db: 'x' }), { database: 'x' })
  assert.deepEqual(parseQueryPrefill({}), {})
})

test('parseAuditPrefill', () => {
  assert.deepEqual(parseAuditPrefill({ range: '7d', action: 'login', q: 'fail', user: 'admin' }), {
    range: '7d',
    action: 'login',
    q: 'fail',
    user: 'admin',
  })
  assert.deepEqual(parseAuditPrefill({ filter: 'x' }), { q: 'x' })
})

test('build prefill paths are read-only deep links', () => {
  assert.equal(buildQueryPrefillPath({ range: '1h' }), '/query?range=1h#query-form')
  assert.equal(buildAuditPrefillPath({ range: '24h', action: 'write' }), '/audit?range=24h&action=write#audit-filters')
  assert.match(buildQueryPrefillPath({ range: '1h' }), /range=1h/)
  assert.doesNotMatch(buildQueryPrefillPath({ range: '1h' }), /execute|auto/)
})

test('write prefill path and parse', () => {
  assert.equal(
    buildWritePrefillPath({ database: 'metrics', measurement: 'cpu' }),
    '/write?database=metrics&measurement=cpu#write-mode-typed',
  )
  assert.deepEqual(parseWritePrefill({ database: 'metrics', measurement: 'cpu' }), {
    database: 'metrics',
    measurement: 'cpu',
  })
})

test('query prefill supports tags/fields and share helper', () => {
  const path = buildQueryPrefillPath({
    database: 'metrics',
    measurement: 'cpu',
    tags: 'host=a',
    fields: 'usage',
    range: '1h',
  })
  assert.match(path, /database=metrics/)
  assert.match(path, /tags=host%3Da|tags=host=a/)
  const parsed = parseQueryPrefill({
    database: 'metrics',
    measurement: 'cpu',
    tags: 'host=a',
    fields: 'usage',
    range: '1h',
  })
  assert.equal(parsed.tags, 'host=a')
  assert.equal(parsed.fields, 'usage')
  assert.equal(
    queryFormToPrefill({ database: 'm', measurement: 'cpu', tags: 'host=a' }),
    '/query?database=m&measurement=cpu&tags=host%3Da#query-form',
  )
})
