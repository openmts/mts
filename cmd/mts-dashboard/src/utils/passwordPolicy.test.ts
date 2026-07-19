import assert from 'node:assert/strict'
import test from 'node:test'
import { validateNewPassword } from './passwordPolicy.ts'

test('validateNewPassword rejects empty short default mismatch same', () => {
  assert.equal(validateNewPassword('', 'abcdefgh', 'abcdefgh').ok, false)
  assert.equal(validateNewPassword('old', 'short', 'short').ok, false)
  assert.equal(validateNewPassword('old', 'admin', 'admin').ok, false)
  assert.equal(validateNewPassword('old', 'abcdefgh', 'abcdeffff').ok, false)
  assert.equal(validateNewPassword('samepass1', 'samepass1', 'samepass1').ok, false)
  assert.equal(validateNewPassword('oldpass12', 'newpass12', 'newpass12').ok, true)
})

test('validateNewPassword en locale', () => {
  const empty = validateNewPassword('', 'x', 'x', { locale: 'en' })
  assert.equal(empty.ok, false)
  assert.match(String(empty.error), /Old and new|required/i)
  const short = validateNewPassword('old', 'short', 'short', { locale: 'en' })
  assert.equal(short.ok, false)
  assert.match(String(short.error), /at least/i)
})
