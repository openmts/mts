import assert from 'node:assert/strict'
import test from 'node:test'
import { buildClientPrefsExport, formatClientPrefsExportPretty } from './clientPrefsExport.ts'

test('buildClientPrefsExport shape', () => {
  const out = buildClientPrefsExport(
    { landing_path: '/query', density: 'compact', locale: 'en', theme: 'dark', sidebar_collapsed: true },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.client.prefs')
  assert.equal(out.version, 1)
  assert.equal(out.generated_at, '2026-07-20T00:00:00.000Z')
  assert.equal(out.prefs.landing_path, '/query')
  assert.equal(out.prefs.density, 'compact')
  assert.ok(!('token' in out))
  assert.ok(!('password' in out.prefs as object))
})

test('formatClientPrefsExportPretty', () => {
  assert.match(formatClientPrefsExportPretty({ landing_path: '/' }), /mts\.client\.prefs/)
})
