import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DASHBOARD_SHORTCUTS,
  isEditableTarget,
  matchNotifyHistoryOpen,
  matchShortcutHelpOpen,
  matchSidebarFilterFocus,
} from './keyboardShortcuts.ts'

test('shortcut catalog non-empty', () => {
  assert.ok(DASHBOARD_SHORTCUTS.length >= 2)
  assert.ok(DASHBOARD_SHORTCUTS.some((s) => s.id === 'palette'))
  assert.ok(DASHBOARD_SHORTCUTS.some((s) => s.id === 'nav-filter'))
  assert.ok(DASHBOARD_SHORTCUTS.some((s) => s.id === 'notify-history'))
})

test('matchShortcutHelpOpen ignores editable', () => {
  const e = { key: '?', metaKey: false, ctrlKey: false, altKey: false, shiftKey: false } as KeyboardEvent
  assert.equal(matchShortcutHelpOpen(e, false), true)
  assert.equal(matchShortcutHelpOpen(e, true), false)
})

test('matchSidebarFilterFocus only bare slash', () => {
  const base = { metaKey: false, ctrlKey: false, altKey: false, shiftKey: false }
  assert.equal(matchSidebarFilterFocus({ ...base, key: '/' } as KeyboardEvent, false), true)
  assert.equal(matchSidebarFilterFocus({ ...base, key: '/' } as KeyboardEvent, true), false)
  assert.equal(matchSidebarFilterFocus({ ...base, key: '/', shiftKey: true } as KeyboardEvent, false), false)
  assert.equal(matchSidebarFilterFocus({ ...base, key: 'k', ctrlKey: true } as KeyboardEvent, false), false)
})

test('matchNotifyHistoryOpen ctrl/cmd shift h', () => {
  assert.equal(
    matchNotifyHistoryOpen({ key: 'h', metaKey: false, ctrlKey: true, altKey: false, shiftKey: true } as KeyboardEvent),
    true,
  )
  assert.equal(
    matchNotifyHistoryOpen({ key: 'H', metaKey: true, ctrlKey: false, altKey: false, shiftKey: true } as KeyboardEvent),
    true,
  )
  assert.equal(
    matchNotifyHistoryOpen({ key: 'h', metaKey: false, ctrlKey: true, altKey: false, shiftKey: false } as KeyboardEvent),
    false,
  )
})

test('isEditableTarget', () => {
  assert.equal(isEditableTarget(null), false)
})
