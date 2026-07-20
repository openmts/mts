import assert from 'node:assert/strict'
import test from 'node:test'
import {
  COMMAND_ACTION_ITEMS,
  COMMAND_NAV_ITEMS,
  allVisibleCommandItems,
  auditRangeToLocalInputs,
  commandItemIndexMap,
  commandListKeyFromEvent,
  filterAuditEvents,
  filterCommandItems,
  flattenCommandGroups,
  groupCommandItems,
  collapseNavItemsForEmptyQuery,
  applyEmptyQueryNavCollapse,
  isCommandDeepLink,
  isCommandAction,
  matchCommandPaletteOpen,
  moveCommandActiveIndex,
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
    'readiness-export-preflight',
    'readiness-deploy-drill',
    'operations-flush',
    'operations-compact',
    'operations-retention',
    'operations-action-log',
    'operations-maint-errors',
    'metrics-summary',
    'metrics-list',
    'config-effective',
    'config-schema',
    'config-error-codes',
    'downsample-filters',
    'downsample-status',
    'audit-filters',
    'audit-table',
    'audit-range-1h',
    'audit-range-24h',
    'audit-action-login',
    'query-form',
    'query-history',
    'query-results',
    'query-chart',
    'query-range-1h',
    'query-range-24h',
    'query-range-7d',
    'write-mode-typed',
    'write-mode-line',
    'write-mode-form',
    'write-mode-prometheus',
    'write-actions',
  ]) {
    assert.ok(admin.some((i) => i.id === id), id)
  }
  const resolve = (k: string) => k
  assert.ok(filterCommandItems(admin, 'signoff', resolve).some((i) => i.id === 'readiness-signoff'))
  assert.ok(filterCommandItems(admin, 'deploy kit', resolve).some((i) => i.id === 'readiness-deploy-kit'))
  assert.ok(filterCommandItems(admin, '#data-restore', resolve).some((i) => i.id === 'storage-data-restore'))
  assert.ok(filterCommandItems(admin, 'memtable', resolve).some((i) => i.id === 'operations-flush'))
  assert.ok(filterCommandItems(admin, 'ops log', resolve).some((i) => i.id === 'operations-action-log'))
  assert.ok(filterCommandItems(admin, 'effective config', resolve).some((i) => i.id === 'config-effective'))
  assert.ok(filterCommandItems(admin, 'audit filters', resolve).some((i) => i.id === 'audit-filters'))
  assert.ok(filterCommandItems(admin, 'watermark', resolve).some((i) => i.id === 'downsample-status'))
  assert.ok(filterCommandItems(admin, 'typed batch', resolve).some((i) => i.id === 'write-mode-typed'))
  assert.ok(filterCommandItems(admin, 'query history', resolve).some((i) => i.id === 'query-history'))
  const user = visibleCommandItems(COMMAND_NAV_ITEMS, false)
  assert.ok(user.every((i) => !i.adminOnly))
})

test('non-admin query write deep links visible', () => {
  const user = visibleCommandItems(COMMAND_NAV_ITEMS, false)
  for (const id of ['query-history', 'write-mode-typed', 'write-actions']) {
    assert.ok(user.some((i) => i.id === id), id)
  }
})

test('command actions catalog and filter', () => {
  assert.ok(COMMAND_ACTION_ITEMS.length >= 10)
  assert.ok(COMMAND_ACTION_ITEMS.some((i) => i.id === 'action-scroll-main-to-top'))
  assert.ok(COMMAND_ACTION_ITEMS.some((i) => i.id === 'action-copy-page-url'))
  assert.ok(COMMAND_ACTION_ITEMS.some((i) => i.id === 'action-focus-main'))
  assert.ok(COMMAND_ACTION_ITEMS.some((i) => i.id === 'action-reload-page'))
  assert.ok(COMMAND_ACTION_ITEMS.every((i) => isCommandAction(i)))
  const all = allVisibleCommandItems(true)
  assert.ok(all.some((i) => i.id === 'action-toggle-theme'))
  assert.ok(all.some((i) => i.id === 'query'))
  const resolve = (k: string) => k
  const themeHits = filterCommandItems(all, 'theme', resolve)
  assert.ok(themeHits.some((i) => i.id === 'action-toggle-theme'))
  const densityHits = filterCommandItems(all, '密度', resolve)
  assert.ok(densityHits.some((i) => i.id === 'action-toggle-density'))
  assert.ok(filterCommandItems(all, '返回顶部', resolve).some((i) => i.id === 'action-scroll-main-to-top'))
  assert.ok(filterCommandItems(all, '复制链接', resolve).some((i) => i.id === 'action-copy-page-url'))
  assert.ok(filterCommandItems(all, 'reload', resolve).some((i) => i.id === 'action-reload-page'))
})

test('groupCommandItems splits nav and action', () => {
  const all = allVisibleCommandItems(true)
  const groups = groupCommandItems(all)
  assert.ok(groups.some((g) => g.id === 'nav'))
  assert.ok(groups.some((g) => g.id === 'action'))
  const nav = groups.find((g) => g.id === 'nav')!
  const action = groups.find((g) => g.id === 'action')!
  assert.ok(nav.items.every((i) => !isCommandAction(i)))
  assert.ok(action.items.every((i) => isCommandAction(i)))
  const flat = flattenCommandGroups(groups)
  assert.equal(flat.length, all.length)
  assert.ok(flat.findIndex((i) => isCommandAction(i)) > flat.findIndex((i) => !isCommandAction(i)))
  const themeOnly = filterCommandItems(all, 'theme', (k) => k)
  const g2 = groupCommandItems(themeOnly)
  assert.ok(g2.every((g) => g.id === 'action' || g.items.length))
  assert.ok(g2.some((g) => g.id === 'action'))
  assert.ok(!g2.some((g) => g.id === 'nav') || g2.find((g) => g.id === 'nav')!.items.length === 0)
  // empty nav group omitted
  assert.equal(g2.filter((g) => g.id === 'nav').length, 0)
})

test('moveCommandActiveIndex and index map', () => {
  assert.equal(moveCommandActiveIndex(0, 0, 'next'), 0)
  assert.equal(moveCommandActiveIndex(0, 3, 'next'), 1)
  assert.equal(moveCommandActiveIndex(2, 3, 'next'), 0)
  assert.equal(moveCommandActiveIndex(0, 3, 'prev'), 2)
  assert.equal(moveCommandActiveIndex(1, 3, 'home'), 0)
  assert.equal(moveCommandActiveIndex(0, 3, 'end'), 2)
  const map = commandItemIndexMap([{ id: 'a' }, { id: 'b' }, { id: 'a' }])
  assert.equal(map.get('a'), 0)
  assert.equal(map.get('b'), 1)
  const e = { key: 'Home' } as KeyboardEvent
  assert.equal(commandListKeyFromEvent(e), 'home')
  assert.equal(commandListKeyFromEvent({ key: 'End' } as KeyboardEvent), 'end')
  assert.equal(commandListKeyFromEvent({ key: 'ArrowDown' } as KeyboardEvent), 'next')
  assert.equal(commandListKeyFromEvent({ key: 'x' } as KeyboardEvent), 'none')
})

test('empty query nav collapse', () => {
  const all = allVisibleCommandItems(true)
  const groups = groupCommandItems(all)
  const nav = groups.find((g) => g.id === 'nav')!
  assert.ok(nav.items.some((i) => isCommandDeepLink(i)))
  const collapsed = collapseNavItemsForEmptyQuery(nav.items, false)
  assert.ok(collapsed.hiddenCount > 0)
  assert.ok(collapsed.items.every((i) => !isCommandDeepLink(i)))
  assert.ok(collapsed.items.some((i) => i.id === 'query'))
  const expanded = collapseNavItemsForEmptyQuery(nav.items, true)
  assert.equal(expanded.hiddenCount, 0)
  assert.ok(expanded.deepLinkCount > 0)
  assert.equal(expanded.items.length, nav.items.length)

  const applied = applyEmptyQueryNavCollapse(groups, false)
  assert.ok(applied.navHiddenCount > 0)
  assert.ok(applied.navDeepLinkCount > 0)
  const appliedOpen = applyEmptyQueryNavCollapse(groups, true)
  assert.equal(appliedOpen.navHiddenCount, 0)
  assert.equal(appliedOpen.navDeepLinkCount, applied.navDeepLinkCount)
  const flat = flattenCommandGroups(applied.groups)
  assert.ok(flat.every((i) => isCommandAction(i) || !isCommandDeepLink(i)))
  assert.ok(flat.some((i) => i.id === 'action-toggle-theme'))

  const withQuery = filterCommandItems(all, 'flush', (k) => k)
  const qGroups = groupCommandItems(withQuery)
  // 有查询时 UI 不折叠；纯函数仍可折叠但 caller 只在 empty query 调用
  assert.ok(qGroups.some((g) => g.items.some((i) => i.id === 'operations-flush')))
})


test('command palette prefill deep links are read-only paths', () => {
  const admin = visibleCommandItems(COMMAND_NAV_ITEMS, true)
  const q1 = admin.find((i) => i.id === 'query-range-1h')
  assert.ok(q1)
  assert.match(q1!.path, /\/query\?range=1h/)
  assert.doesNotMatch(q1!.path, /execute|auto-run/)
  const a1 = admin.find((i) => i.id === 'audit-range-24h')
  assert.ok(a1)
  assert.match(a1!.path, /\/audit\?range=24h/)
  assert.equal(a1!.adminOnly, true)
  const login = admin.find((i) => i.id === 'audit-action-login')
  assert.ok(login)
  assert.match(login!.path, /action=login/)
})
