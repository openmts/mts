import assert from 'node:assert/strict'
import test from 'node:test'
import { permissionLabel, DB_PERMISSIONS } from './permissionLabel.ts'

test('permissionLabel localizes known perms', () => {
  assert.equal(permissionLabel('read', 'zh'), '读')
  assert.equal(permissionLabel('write', 'en'), 'Write')
  assert.equal(permissionLabel('admin', 'zh'), '管理')
  assert.equal(permissionLabel('custom', 'en'), 'custom')
  assert.ok(DB_PERMISSIONS.includes('read'))
})
