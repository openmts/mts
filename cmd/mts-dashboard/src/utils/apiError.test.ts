import assert from 'node:assert/strict'
import test from 'node:test'
import {
  errorCodeFromStatus,
  formatCaughtError,
  friendlyApiError,
  normalizeErrorCode,
  resolveErrorCode,
} from './apiError.ts'

test('normalizeErrorCode defaults', () => {
  assert.equal(normalizeErrorCode(''), 'internal')
  assert.equal(normalizeErrorCode('Permission_Denied'), 'permission_denied')
})

test('errorCodeFromStatus maps common HTTP statuses', () => {
  assert.equal(errorCodeFromStatus(400), 'bad_request')
  assert.equal(errorCodeFromStatus(401), 'unauthenticated')
  assert.equal(errorCodeFromStatus(403), 'permission_denied')
  assert.equal(errorCodeFromStatus(404), 'not_found')
  assert.equal(errorCodeFromStatus(409), 'already_exists')
  assert.equal(errorCodeFromStatus(429), 'resource_exhausted')
  assert.equal(errorCodeFromStatus(408), 'timeout')
  assert.equal(errorCodeFromStatus(499), 'canceled')
  assert.equal(errorCodeFromStatus(500), 'internal')
  assert.equal(errorCodeFromStatus(200), null)
})

test('resolveErrorCode prefers known code then status', () => {
  assert.equal(resolveErrorCode('permission_denied', 500), 'permission_denied')
  assert.equal(resolveErrorCode('', 404), 'not_found')
  assert.equal(resolveErrorCode('weird_custom', 403), 'permission_denied')
  assert.equal(resolveErrorCode(undefined, undefined), 'internal')
})

test('friendlyApiError maps known codes without bare code prefix', () => {
  const e = friendlyApiError({ code: 'permission_denied', message: 'nope' }, 'zh')
  assert.equal(e.code, 'permission_denied')
  assert.match(e.display, /权限不足/)
  assert.match(e.display, /nope/)
  assert.doesNotMatch(e.display, /^\[permission_denied\]/)
  const en = friendlyApiError({ code: 'unauthenticated' }, 'en')
  assert.match(en.display, /Unauthenticated|sign in/i)
  assert.doesNotMatch(en.display, /^\[unauthenticated\]/i)
})

test('friendlyApiError ignores bare code as message', () => {
  const e = friendlyApiError({ code: 'not_found', message: 'not_found' }, 'zh')
  assert.match(e.display, /资源不存在/)
  assert.doesNotMatch(e.display, /（not_found）/)
})

test('formatCaughtError handles APIClientError-like and network', () => {
  const api = { name: 'APIClientError', code: 'not_found', message: 'missing', status: 404 }
  assert.match(formatCaughtError(api, 'zh'), /资源不存在/)
  assert.match(formatCaughtError(new Error('Failed to fetch'), 'en'), /Network|reach server/i)
  const statusOnly = { name: 'APIClientError', status: 403, message: 'no' }
  assert.match(formatCaughtError(statusOnly, 'zh'), /权限不足/)
})

test('friendlyApiError timeout', () => {
  const e = friendlyApiError({ code: 'timeout', status: 408 }, 'zh')
  assert.match(e.display, /超时/)
})
