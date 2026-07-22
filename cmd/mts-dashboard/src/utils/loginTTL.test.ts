import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyServerPasswordPolicy,
  resetPasswordPolicyRuntime,
} from './passwordPolicy.ts'
import { formatAuthTTLLimit, parseLoginTTLSeconds } from './loginTTL.ts'

test('parseLoginTTLSeconds empty uses server default', () => {
  resetPasswordPolicyRuntime()
  assert.deepEqual(parseLoginTTLSeconds(''), { ok: true })
  assert.deepEqual(parseLoginTTLSeconds('   '), { ok: true })
})

test('parseLoginTTLSeconds accepts positive int', () => {
  resetPasswordPolicyRuntime()
  assert.deepEqual(parseLoginTTLSeconds('3600'), { ok: true, seconds: 3600 })
  assert.deepEqual(parseLoginTTLSeconds('1'), { ok: true, seconds: 1 })
})

test('parseLoginTTLSeconds rejects invalid', () => {
  resetPasswordPolicyRuntime()
  assert.deepEqual(parseLoginTTLSeconds('0'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('-1'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('1.5'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('abc'), { ok: false, reason: 'invalid' })
})

test('parseLoginTTLSeconds rejects too large vs runtime max', () => {
  resetPasswordPolicyRuntime()
  applyServerPasswordPolicy({ max_auth_ttl_seconds: 3600 })
  assert.deepEqual(parseLoginTTLSeconds('3601'), { ok: false, reason: 'too_large' })
  assert.deepEqual(parseLoginTTLSeconds('3600'), { ok: true, seconds: 3600 })
  resetPasswordPolicyRuntime()
})

test('formatAuthTTLLimit', () => {
  assert.equal(formatAuthTTLLimit(30 * 24 * 3600), '30d')
  assert.equal(formatAuthTTLLimit(12 * 3600), '12h')
  assert.match(formatAuthTTLLimit(90), /1m|90s/)
})
