import assert from 'node:assert/strict'
import test from 'node:test'
import { isEditableTarget, matchQueryShortcut } from './keyboard.ts'

function key(init: Partial<KeyboardEvent> & { key: string }): KeyboardEvent {
  return {
    key: init.key,
    metaKey: Boolean(init.metaKey),
    ctrlKey: Boolean(init.ctrlKey),
    shiftKey: Boolean(init.shiftKey),
    altKey: Boolean(init.altKey),
  } as KeyboardEvent
}

test('matchQueryShortcut maps primary combos', () => {
  assert.equal(matchQueryShortcut(key({ key: 'Enter', ctrlKey: true })), 'run')
  assert.equal(matchQueryShortcut(key({ key: 'Enter', metaKey: true })), 'run')
  assert.equal(matchQueryShortcut(key({ key: 'Escape' })), 'cancel')
  assert.equal(matchQueryShortcut(key({ key: 'h', ctrlKey: true })), 'toggle-history')
  assert.equal(matchQueryShortcut(key({ key: 'C', ctrlKey: true, shiftKey: true })), 'copy')
  assert.equal(matchQueryShortcut(key({ key: 'c', ctrlKey: true })), null)
  assert.equal(matchQueryShortcut(key({ key: 'Enter' })), null)
})

test('isEditableTarget is false for non-elements', () => {
  assert.equal(isEditableTarget(null), false)
})
