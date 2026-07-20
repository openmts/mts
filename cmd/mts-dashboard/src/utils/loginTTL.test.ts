import assert from 'node:assert/strict'
import test from 'node:test'
import { parseLoginTTLSeconds } from './loginTTL.ts'

test('parseLoginTTLSeconds empty uses server default', () => {
  assert.deepEqual(parseLoginTTLSeconds(''), { ok: true })
  assert.deepEqual(parseLoginTTLSeconds('   '), { ok: true })
})

test('parseLoginTTLSeconds accepts positive int', () => {
  assert.deepEqual(parseLoginTTLSeconds('3600'), { ok: true, seconds: 3600 })
  assert.deepEqual(parseLoginTTLSeconds('1'), { ok: true, seconds: 1 })
})

test('parseLoginTTLSeconds rejects invalid', () => {
  assert.deepEqual(parseLoginTTLSeconds('0'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('-1'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('1.5'), { ok: false, reason: 'invalid' })
  assert.deepEqual(parseLoginTTLSeconds('abc'), { ok: false, reason: 'invalid' })
})
