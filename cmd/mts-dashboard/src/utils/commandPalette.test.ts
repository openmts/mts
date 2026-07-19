import assert from 'node:assert/strict'
import test from 'node:test'
import {
  COMMAND_NAV_ITEMS,
  auditRangeToLocalInputs,
  filterAuditEvents,
  filterCommandItems,
  matchCommandPaletteOpen,
  visibleCommandItems,
} from './commandPalette.ts'

test('visibleCommandItems hides admin for non-admin', () => {
  const user = visibleCommandItems(COMMAND_NAV_ITEMS, false)
  assert.ok(user.every((i) => !i.adminOnly))
  assert.ok(user.some((i) => i.id === 'query'))
  const admin = visibleCommandItems(COMMAND_NAV_ITEMS, true)
  assert.ok(admin.some((i) => i.id === 'storage'))
})

test('filterCommandItems matches keywords path label', () => {
  const resolve = (k: string) => (k === 'query' ? '查询' : k)
  const items = visibleCommandItems(COMMAND_NAV_ITEMS, true)
  assert.ok(filterCommandItems(items, 'flush', resolve).some((i) => i.id === 'operations'))
  assert.ok(filterCommandItems(items, '查询', resolve).some((i) => i.id === 'query'))
  assert.ok(filterCommandItems(items, '/storage', resolve).some((i) => i.id === 'storage'))
})

test('matchCommandPaletteOpen ctrl/meta k', () => {
  const e = { key: 'k', ctrlKey: true, metaKey: false, altKey: false, shiftKey: false } as KeyboardEvent
  assert.equal(matchCommandPaletteOpen(e, false), true)
  const no = { key: 'k', ctrlKey: false, metaKey: false, altKey: false, shiftKey: false } as KeyboardEvent
  assert.equal(matchCommandPaletteOpen(no, false), false)
})

test('auditRangeToLocalInputs and filterAuditEvents', () => {
  const r = auditRangeToLocalInputs('1h', new Date('2026-07-20T12:00:00'))
  assert.ok(r.since)
  assert.ok(r.until)
  const clear = auditRangeToLocalInputs('clear')
  assert.equal(clear.since, '')
  const events = [
    { action: 'login', detail: 'ok', user_name: 'a' },
    { action: 'flush', detail: 'done', user_name: 'b' },
  ]
  assert.equal(filterAuditEvents(events, 'flush').length, 1)
  assert.equal(filterAuditEvents(events, '').length, 2)
})

test('command palette includes ops deep links for admin', () => {
  const admin = visibleCommandItems(COMMAND_NAV_ITEMS, true)
  for (const id of [
    'storage-data-restore',
    'storage-backup-drill',
    'storage-edge-https',
    'readiness-deploy-kit',
    'readiness-signoff',
  ]) {
    assert.ok(admin.some((i) => i.id === id), id)
  }
  const resolve = (k: string) => k
  assert.ok(filterCommandItems(admin, 'signoff', resolve).some((i) => i.id === 'readiness-signoff'))
  assert.ok(filterCommandItems(admin, 'deploy kit', resolve).some((i) => i.id === 'readiness-deploy-kit'))
  assert.ok(filterCommandItems(admin, '#data-restore', resolve).some((i) => i.id === 'storage-data-restore'))
  const user = visibleCommandItems(COMMAND_NAV_ITEMS, false)
  assert.ok(user.every((i) => !i.id.startsWith('readiness') && !i.id.startsWith('storage-')))
})
