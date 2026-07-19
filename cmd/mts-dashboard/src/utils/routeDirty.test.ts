import assert from 'node:assert/strict'
import test from 'node:test'
import {
  allowNavigationWhenDirty,
  anyRouteDirty,
  clearDirtyCheckers,
  dirtyCheckerCount,
  registerDirtyChecker,
} from './routeDirty.ts'

test('registerDirtyChecker tracks dirty state', () => {
  clearDirtyCheckers()
  let dirty = false
  const off = registerDirtyChecker('q', () => dirty)
  assert.equal(anyRouteDirty(), false)
  dirty = true
  assert.equal(anyRouteDirty(), true)
  assert.equal(dirtyCheckerCount(), 1)
  off()
  assert.equal(anyRouteDirty(), false)
  assert.equal(dirtyCheckerCount(), 0)
})

test('allowNavigationWhenDirty confirms only when dirty', () => {
  let asked = 0
  assert.equal(
    allowNavigationWhenDirty(false, 'leave?', () => {
      asked++
      return false
    }),
    true,
  )
  assert.equal(asked, 0)
  assert.equal(
    allowNavigationWhenDirty(true, 'leave?', () => {
      asked++
      return false
    }),
    false,
  )
  assert.equal(asked, 1)
  assert.equal(
    allowNavigationWhenDirty(true, 'leave?', () => {
      asked++
      return true
    }),
    true,
  )
})
