import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_CLIENT_PREFS,
  normalizeClientPrefs,
  parseClientPrefsImport,
} from './clientPrefs.ts'

test('normalizeClientPrefs defaults and clamps', () => {
  assert.deepEqual(normalizeClientPrefs({}), DEFAULT_CLIENT_PREFS)
  const n = normalizeClientPrefs({
    landing_path: '/query',
    density: 'compact',
    sidebar_collapsed: true,
    locale: 'en',
    theme: 'dark',
  })
  assert.equal(n.landing_path, '/query')
  assert.equal(n.density, 'compact')
  assert.equal(n.sidebar_collapsed, true)
  assert.equal(n.locale, 'en')
  assert.equal(n.theme, 'dark')
  assert.deepEqual(n.nav_order, {})
  assert.equal(normalizeClientPrefs({ landing_path: '/login' }).landing_path, '/')
  const withNav = normalizeClientPrefs({
    nav_order: { workspace: ['/write', '/query', '/'] },
  })
  assert.deepEqual(withNav.nav_order, { workspace: ['/write', '/query', '/'] })
})

test('parseClientPrefsImport from snapshot and flat', () => {
  const snap = parseClientPrefsImport(JSON.stringify({
    kind: 'mts.account.snapshot',
    prefs: { landing_path: '/write', density: 'compact', locale: 'en', theme: 'dark', sidebar_collapsed: true },
  }))
  assert.equal(snap.ok, true)
  if (snap.ok) {
    assert.equal(snap.prefs.landing_path, '/write')
    assert.equal(snap.prefs.density, 'compact')
  }
  const flat = parseClientPrefsImport(JSON.stringify({ landing_path: '/about', density: 'comfortable' }))
  assert.equal(flat.ok, true)
  if (flat.ok) assert.equal(flat.prefs.landing_path, '/about')
  assert.equal(parseClientPrefsImport('').ok, false)
  assert.equal(parseClientPrefsImport('{').ok, false)
  assert.equal(parseClientPrefsImport('{"x":1}').ok, false)
})

test('parseClientPrefsImport from client prefs export', () => {
  const raw = JSON.stringify({
    kind: 'mts.client.prefs',
    version: 1,
    prefs: {
      landing_path: '/audit',
      density: 'compact',
      locale: 'zh',
      theme: 'light',
      sidebar_collapsed: false,
      nav_order: { workspace: ['/query', '/write', '/'] },
    },
  })
  const r = parseClientPrefsImport(raw)
  assert.equal(r.ok, true)
  if (r.ok) {
    assert.equal(r.prefs.landing_path, '/audit')
    assert.deepEqual(r.prefs.nav_order, { workspace: ['/query', '/write', '/'] })
  }
})

test('parseClientPrefsImport nav_order only shape', () => {
  const r = parseClientPrefsImport(JSON.stringify({ nav_order: { admin: ['/audit'] } }))
  assert.equal(r.ok, true)
  if (r.ok) assert.deepEqual(r.prefs.nav_order, { admin: ['/audit'] })
})
