import assert from 'node:assert/strict'
import test from 'node:test'
import {
  errorCodeFromStatus,
  formatCaughtError,
  friendlyApiError,
  isCanceledError,
  isTimeoutError,
  normalizeErrorCode,
  resolveCaughtErrorCode,
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

test('resolveCaughtErrorCode maps AbortError to canceled not timeout', () => {
  const abort = new Error('The user aborted a request.')
  abort.name = 'AbortError'
  assert.equal(resolveCaughtErrorCode(abort), 'canceled')
  assert.equal(isCanceledError(abort), true)
  assert.equal(isTimeoutError(abort), false)
  assert.match(formatCaughtError(abort, 'zh'), /取消/)
  assert.doesNotMatch(formatCaughtError(abort, 'zh'), /超时/)
})

test('resolveCaughtErrorCode maps APIClientError timeout and canceled', () => {
  const timeout = { name: 'APIClientError', code: 'timeout', status: 408, message: 'request timeout' }
  const canceled = { name: 'APIClientError', code: 'canceled', status: 499, message: 'request canceled' }
  assert.equal(resolveCaughtErrorCode(timeout), 'timeout')
  assert.equal(isTimeoutError(timeout), true)
  assert.equal(resolveCaughtErrorCode(canceled), 'canceled')
  assert.equal(isCanceledError(canceled), true)
  assert.match(formatCaughtError(timeout, 'zh'), /超时/)
  assert.match(formatCaughtError(canceled, 'zh'), /取消/)
  // 传输层噪声 message 不拼入主文案
  assert.doesNotMatch(formatCaughtError(canceled, 'zh'), /request canceled/i)
})

test('formatCaughtError timed-out Error message is timeout', () => {
  assert.equal(resolveCaughtErrorCode(new Error('request timed out')), 'timeout')
  assert.match(formatCaughtError(new Error('request timed out'), 'en'), /timeout/i)
})

test('resolveCaughtErrorCode maps localized cancel and timeout strings', () => {
  assert.equal(resolveCaughtErrorCode('请求已取消'), 'canceled')
  assert.equal(isCanceledError('admin action cancelled'), true)
  assert.equal(resolveCaughtErrorCode('操作超时，请稍后重试'), 'timeout')
  assert.equal(isTimeoutError('request timed out'), true)
})

test('friendlyApiError admin heavy resource_exhausted', () => {
  const e = friendlyApiError(
    { code: 'resource_exhausted', status: 429, message: 'admin heavy operation already in progress' },
    'zh',
  )
  assert.match(e.display, /管理重操作/)
  assert.doesNotMatch(e.display, /请求超过限制/)
  const withOp = friendlyApiError(
    { code: 'resource_exhausted', status: 429, message: 'admin heavy operation already in progress: flush' },
    'zh',
  )
  assert.match(withOp.display, /flush/)
  const en = friendlyApiError(
    { code: 'resource_exhausted', status: 429, message: 'admin heavy operation already in progress' },
    'en',
  )
  assert.match(en.display, /Admin operation busy/i)
})

test('friendlyApiError structured adminOpBusy prefers op field', () => {
  const f = friendlyApiError({
    code: 'resource_exhausted',
    status: 429,
    message: 'admin heavy operation already in progress',
    adminOpBusy: true,
    op: 'retention',
  })
  assert.match(f.display, /retention/)
  assert.match(f.title, /管理|Admin/)
})

test('friendlyApiError change password invalid credentials', () => {
  const e = friendlyApiError(
    { code: 'bad_request', status: 400, message: 'invalid credentials' },
    'zh',
  )
  assert.match(e.title, /修改密码/)
  assert.match(e.display, /当前密码不正确|会话仍然有效/)
  const en = friendlyApiError(
    { code: 'bad_request', status: 400, message: 'invalid credentials' },
    'en',
  )
  assert.match(en.display, /incorrect|session remains/i)
})

