import assert from 'node:assert/strict'
import test from 'node:test'
import {
  alignAccountSession,
  AUTH_SESSION_PATH,
  AUTH_PASSWORD_POLICY_PATH,
} from './accountSessionAlign.ts'

test('alignAccountSession ok', () => {
  const a = alignAccountSession({
    sampleSource: 'session',
    hasServerRemaining: true,
    hasLocalExpiry: true,
    urgency: 'ok',
  })
  assert.equal(a.session_path, AUTH_SESSION_PATH)
  assert.equal(a.password_policy_path, AUTH_PASSWORD_POLICY_PATH)
  assert.equal(a.tone, 'ok')
  assert.equal(a.path_ok, true)
})

test('alignAccountSession warn without server remaining', () => {
  const a = alignAccountSession({
    hasServerRemaining: false,
    hasLocalExpiry: true,
    urgency: 'ok',
  })
  assert.equal(a.tone, 'warn')
})

test('alignAccountSession critical is bad', () => {
  const a = alignAccountSession({ urgency: 'critical', hasLocalExpiry: true, hasServerRemaining: true })
  assert.equal(a.tone, 'bad')
})
