import assert from 'node:assert/strict'
import test from 'node:test'
import { buildAccountExport, formatAccountExportPretty } from './accountExport.ts'

test('buildAccountExport omits secrets', () => {
  const out = buildAccountExport(
    {
      username: 'admin',
      role: 'admin',
      session: { expires_at: 't', remaining: '1h', urgency: 'ok' },
    },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.account.snapshot')
  assert.equal(out.username, 'admin')
  assert.equal(out.session.urgency, 'ok')
  assert.ok(!('password' in out))
})

test('formatAccountExportPretty', () => {
  assert.match(formatAccountExportPretty({ username: 'u' }), /mts.account.snapshot/)
})

test('buildAccountExport includes prefs', () => {
  const out = buildAccountExport({
    username: 'a',
    role: 'admin',
    prefs: { landing_path: '/query', density: 'compact', sidebar_collapsed: true, locale: 'en', theme: 'dark' },
  })
  assert.equal(out.prefs.landing_path, '/query')
  assert.equal(out.prefs.density, 'compact')
  assert.equal(out.prefs.sidebar_collapsed, true)
})

test('buildAccountExport includes nav_order', () => {
  const out = buildAccountExport({
    username: 'a',
    role: 'admin',
    prefs: {
      landing_path: '/query',
      density: 'compact',
      sidebar_collapsed: true,
      locale: 'en',
      theme: 'dark',
      nav_order: { workspace: ['/query', '/'] },
    },
  })
  assert.deepEqual(out.prefs.nav_order, { workspace: ['/query', '/'] })
  const empty = buildAccountExport({ username: 'a' })
  assert.deepEqual(empty.prefs.nav_order, {})
})

