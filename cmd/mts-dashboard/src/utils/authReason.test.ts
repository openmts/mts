import assert from 'node:assert/strict'
import test from 'node:test'
import { loginReasonMessage } from './authReason.ts'

test('loginReasonMessage zh/en', () => {
  assert.match(loginReasonMessage('session', 'zh'), /过期/)
  assert.match(loginReasonMessage('storage', 'en'), /another tab/i)
  assert.equal(loginReasonMessage('', 'zh'), '')
  assert.equal(loginReasonMessage(undefined, 'zh'), '')
})

test('loginReasonMessage password_changed', () => {
  assert.ok(loginReasonMessage('password_changed', 'zh').includes('新密码'))
  assert.ok(loginReasonMessage('password_changed', 'en').toLowerCase().includes('password'))
})
