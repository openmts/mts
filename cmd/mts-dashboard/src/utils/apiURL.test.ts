import assert from 'node:assert/strict'
import test from 'node:test'
import { buildAPIURL } from './apiURL.ts'

test('buildAPIURL applies non-empty API base exactly once', () => {
  assert.equal(buildAPIURL('/gateway/', '/api/v1/auth/logout'), '/gateway/api/v1/auth/logout')
  assert.equal(buildAPIURL('', '/api/v1/auth/logout'), '/api/v1/auth/logout')
})
