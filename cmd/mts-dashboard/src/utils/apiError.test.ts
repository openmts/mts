import assert from 'node:assert/strict'
import test from 'node:test'
import { formatCaughtError, friendlyApiError, normalizeErrorCode } from './apiError.ts'

test('normalizeErrorCode defaults', () => {
  assert.equal(normalizeErrorCode(''), 'internal')
  assert.equal(normalizeErrorCode('Permission_Denied'), 'permission_denied')
})

test('friendlyApiError maps known codes', () => {
  const e = friendlyApiError({ code: 'permission_denied', message: 'nope' }, 'zh')
  assert.equal(e.code, 'permission_denied')
  assert.match(e.display, /权限不足/)
  assert.match(e.display, /nope/)
  const en = friendlyApiError({ code: 'unauthenticated' }, 'en')
  assert.match(en.display, /Unauthenticated|sign in/i)
})

test('formatCaughtError handles APIClientError-like and network', () => {
  const api = { name: 'APIClientError', code: 'not_found', message: 'missing', status: 404 }
  assert.match(formatCaughtError(api, 'zh'), /资源不存在/)
  assert.match(formatCaughtError(new Error('Failed to fetch'), 'en'), /Network/i)
})
