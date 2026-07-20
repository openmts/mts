import assert from 'node:assert/strict'
import test from 'node:test'
import { buildAboutExport, formatAboutExportPretty } from './aboutExport.ts'

const client = {
  name: 'mts-dashboard',
  version: '0.0.0',
  mode: 'test',
  base: '/',
  apiBase: '',
}

test('buildAboutExport', () => {
  const out = buildAboutExport(
    { client, server: { version: '1.0', commit: 'abc', built_at: 't' }, user: 'admin' },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.about')
  assert.equal(out.user, 'admin')
  assert.equal(out.server?.version, '1.0')
})

test('formatAboutExportPretty', () => {
  const text = formatAboutExportPretty({ client })
  assert.match(text, /mts.about/)
})
