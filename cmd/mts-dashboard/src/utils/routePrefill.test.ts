import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildAuditPrefillPath,
  buildQueryPrefillPath,
  buildWritePrefillPath,
  parseWritePrefill,
  queryFormToPrefill,
  writeFormToPrefill,
  auditFormToPrefill,
  parseDatabasesPrefill,
  buildDatabasesPrefillPath,
  databasesFormToPrefill,
  parseUsersPrefill,
  usersFormToPrefill,
  parseAccessPrefill,
  accessFormToPrefill,
  parseAccessGrantsPrefill,
  accessGrantsFormToPrefill,
  parseDownsamplePrefill,
  downsampleFormToPrefill,
  parseOperationsPrefill,
  operationsFormToPrefill,
  parseStoragePrefill,
  storageFormToPrefill,
  parseReadinessPrefill,
  readinessFormToPrefill,
  parseConfigPrefill,
  configFormToPrefill,
  parseMetricsPrefill,
  metricsFormToPrefill,
  parseApiSpecPrefill,
  apiSpecFormToPrefill,
  parseAccountPrefill,
  accountFormToPrefill,
  parseOverviewPrefill,
  overviewFormToPrefill,
  parseAboutPrefill,
  aboutFormToPrefill,
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

test('query prefill absolute start/end preferred over range', () => {
  const path = queryFormToPrefill({
    database: 'm',
    measurement: 'cpu',
    start_time: '1700000000000',
    end_time: '1700003600000',
  })
  assert.match(path, /start_time=1700000000000/)
  assert.match(path, /end_time=1700003600000/)
  assert.doesNotMatch(path, /range=/)
  const parsed = parseQueryPrefill({
    range: '1h',
    start_time: '1700000000000',
    end_time: '1700003600000',
    database: 'm',
  })
  assert.equal(parsed.start_time, '1700000000000')
  assert.equal(parsed.end_time, '1700003600000')
  assert.equal(parsed.range, undefined)
  assert.equal(parsed.database, 'm')
})

test('write/audit form share helpers', () => {
  assert.equal(
    writeFormToPrefill({ database: 'metrics', measurement: 'cpu' }),
    '/write?database=metrics&measurement=cpu#write-mode-typed',
  )
  assert.equal(
    auditFormToPrefill({ range: '24h', action: 'login', user: 'reader' }),
    '/audit?range=24h&action=login&user=reader#audit-filters',
  )
})

test('databases prefill parse and share', () => {
  assert.deepEqual(parseDatabasesPrefill({ database: 'metrics', q: 'met' }), {
    database: 'metrics',
    q: 'met',
  })
  assert.equal(
    databasesFormToPrefill({ database: 'metrics', q: 'm' }),
    '/databases?database=metrics&q=m#databases-filter-bar',
  )
  assert.match(buildDatabasesPrefillPath({ database: 'd1' }), /database=d1/)
})

test('users and access prefill share helpers', () => {
  assert.deepEqual(parseUsersPrefill({ q: 'alice', role: 'user', user: 'alice' }), {
    q: 'alice',
    role: 'user',
    user: 'alice',
  })
  assert.deepEqual(
    parseUsersPrefill({ q: 'bob', role: 'user', status: 'disabled', user: 'bob' }),
    { q: 'bob', role: 'user', status: 'disabled', user: 'bob' },
  )
  assert.equal(
    usersFormToPrefill({ q: 'a', role: 'admin', user: 'root' }),
    '/users?q=a&role=admin&user=root#users-filter-bar',
  )
  assert.equal(
    usersFormToPrefill({ q: 'x', role: 'user', status: 'active' }),
    '/users?q=x&role=user&status=active#users-filter-bar',
  )
  assert.deepEqual(parseAccessPrefill({ role: 'admin', area: 'access', q: 'audit' }), {
    role: 'admin',
    area: 'access',
    q: 'audit',
  })
  assert.equal(
    accessFormToPrefill({ role: 'user', area: 'workspace', q: 'query' }),
    '/access?role=user&area=workspace&q=query#access-matrix-filter-bar',
  )
})

test('access grants and downsample prefill helpers', () => {
  assert.deepEqual(
    parseAccessGrantsPrefill({ user: 'alice', database: 'metrics', permission: 'read', q: 'cpu' }),
    { user: 'alice', database: 'metrics', permission: 'read', q: 'cpu' },
  )
  assert.equal(
    accessGrantsFormToPrefill({ user: 'alice', database: 'm', permission: 'write' }),
    '/access/grants?user=alice&database=m&permission=write#access-grants-filters',
  )
  assert.deepEqual(parseDownsamplePrefill({ q: 'cpu', enabled: 'enabled' }), {
    q: 'cpu',
    enabled: 'enabled',
  })
  assert.deepEqual(parseDownsamplePrefill({ policy: 'p1' }), { policy: 'p1' })
  assert.equal(
    downsampleFormToPrefill({ q: 'roll', enabled: 'disabled' }),
    '/downsample?q=roll&enabled=disabled#downsample-filters',
  )
  assert.equal(
    downsampleFormToPrefill({ policy: 'e2e-batch-ds' }),
    '/downsample?policy=e2e-batch-ds#downsample-detail',
  )
})

test('operations and storage prefill helpers', () => {
  assert.deepEqual(
    parseOperationsPrefill({ maint_q: 'disk', action_kind: 'flush', action_status: 'error', action_q: 'fail' }),
    { maint_q: 'disk', action_kind: 'flush', action_status: 'error', action_q: 'fail' },
  )
  assert.equal(
    operationsFormToPrefill({ action_kind: 'compact', action_status: 'ok' }),
    '/operations?action_kind=compact&action_status=ok#ops-action-log',
  )
  assert.deepEqual(parseStoragePrefill({}, '#data-restore'), { section: 'data-restore' })
  assert.equal(storageFormToPrefill({ section: 'edge-https' }), '/storage#edge-https')
})

test('readiness config metrics prefill helpers', () => {
  assert.deepEqual(parseReadinessPrefill({}, '#deploy-kit'), { section: 'deploy-kit' })
  assert.equal(readinessFormToPrefill({ section: 'signoff-notes' }), '/ops/readiness#signoff-notes')
  assert.deepEqual(parseConfigPrefill({ schema_q: 'http', error_q: 'auth' }, '#config-schema'), {
    schema_q: 'http',
    error_q: 'auth',
    section: 'config-schema',
  })
  assert.equal(
    configFormToPrefill({ schema_q: 'engine', section: 'config-schema' }),
    '/config?schema_q=engine#config-schema',
  )
  assert.deepEqual(parseMetricsPrefill({ q: 'http', family: 'go_goroutines' }), {
    q: 'http',
    family: 'go_goroutines',
  })
  assert.equal(
    metricsFormToPrefill({ q: 'prom', family: 'go_threads' }),
    '/observability/metrics?q=prom&family=go_threads#metrics-detail',
  )
})

test('apiSpec and account prefill helpers', () => {
  assert.deepEqual(parseApiSpecPrefill({ ns: 'admin', q: 'flush' }), { ns: 'admin', q: 'flush' })
  assert.equal(
    apiSpecFormToPrefill({ ns: 'data', q: 'query' }),
    '/api-spec?ns=data&q=query#api-spec-filters',
  )
  assert.deepEqual(parseAccountPrefill({ landing_q: 'query' }), { landing_q: 'query' })
  assert.equal(
    accountFormToPrefill({ landing_q: 'write' }),
    '/account?landing_q=write#account-landing',
  )
})

test('overview and about prefill helpers', () => {
  assert.deepEqual(parseOverviewPrefill({}, '#overview-health-checks'), { section: 'overview-health-checks' })
  assert.equal(overviewFormToPrefill({ section: 'overview-workspace' }), '/#overview-workspace')
  assert.deepEqual(parseAboutPrefill({}, '#about-server'), { section: 'about-server' })
  assert.equal(aboutFormToPrefill({ section: 'about-client' }), '/about#about-client')
})
