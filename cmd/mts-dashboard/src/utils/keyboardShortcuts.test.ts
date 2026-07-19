import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DASHBOARD_SHORTCUTS,
  isEditableTarget,
  matchShortcutHelpOpen,
} from './keyboardShortcuts.ts'

test('shortcut catalog non-empty', () => {
  assert.ok(DASHBOARD_SHORTCUTS.length >= 2)
  assert.ok(DASHBOARD_SHORTCUTS.some((s) => s.id === 'palette'))
})

test('matchShortcutHelpOpen ignores editable', () => {
  const e = { key: '?', metaKey: false, ctrlKey: false, altKey: false, shiftKey: false } as KeyboardEvent
  assert.equal(matchShortcutHelpOpen(e, false), true)
  assert.equal(matchShortcutHelpOpen(e, true), false)
})

test('isEditableTarget', () => {
  assert.equal(isEditableTarget(null), false)
})
