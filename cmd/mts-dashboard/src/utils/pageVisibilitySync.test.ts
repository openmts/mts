import assert from 'node:assert/strict'
import { test } from 'node:test'
import { shouldSyncOnVisibility } from './pageVisibilitySync.ts'

test('shouldSyncOnVisibility visible only', () => {
  assert.equal(shouldSyncOnVisibility('visible'), true)
  assert.equal(shouldSyncOnVisibility('hidden'), false)
  assert.equal(shouldSyncOnVisibility('prerender'), false)
  assert.equal(shouldSyncOnVisibility(null, false), true)
  assert.equal(shouldSyncOnVisibility(null, true), false)
})
