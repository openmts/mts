import assert from 'node:assert/strict'
import test from 'node:test'
import { clientBuildInfo } from './buildInfo.ts'

test('clientBuildInfo returns stable shape', () => {
  const info = clientBuildInfo()
  assert.equal(info.name, 'mts-dashboard')
  assert.ok(typeof info.version === 'string')
  assert.ok(typeof info.mode === 'string')
  assert.ok(typeof info.base === 'string')
})
