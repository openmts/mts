import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyServerPasswordPolicy,
  getMinPasswordLength,
  resetPasswordPolicyRuntime,
  validateAssignedPassword,
  validateNewPassword,
} from './passwordPolicy.ts'

test('validateNewPassword rejects empty short default mismatch same', () => {
  resetPasswordPolicyRuntime()
  assert.equal(validateNewPassword('', 'abcdefgh', 'abcdefgh').ok, false)
  assert.equal(validateNewPassword('old', 'short', 'short').ok, false)
  assert.equal(validateNewPassword('old', 'admin', 'admin').ok, false)
  assert.equal(validateNewPassword('old', 'abcdefgh', 'abcdeffff').ok, false)
  assert.equal(validateNewPassword('samepass1', 'samepass1', 'samepass1').ok, false)
  assert.equal(validateNewPassword('oldpass12', 'newpass12', 'newpass12').ok, true)
})

test('validateNewPassword en locale', () => {
  resetPasswordPolicyRuntime()
  const empty = validateNewPassword('', 'x', 'x', { locale: 'en' })
  assert.equal(empty.ok, false)
  assert.match(String(empty.error), /Old and new|required/i)
  const short = validateNewPassword('old', 'short', 'short', { locale: 'en' })
  assert.equal(short.ok, false)
  assert.match(String(short.error), /at least/i)
})

test('validateNewPassword requireConfirm false skips confirm match', () => {
  resetPasswordPolicyRuntime()
  const ok = validateNewPassword('oldpass12', 'newpass12', '', { requireConfirm: false })
  assert.equal(ok.ok, true)
  const bad = validateNewPassword('oldpass12', 'short', '', { requireConfirm: false })
  assert.equal(bad.ok, false)
})

test('validateAssignedPassword', () => {
  resetPasswordPolicyRuntime()
  assert.equal(validateAssignedPassword('', { allowEmpty: true }).ok, true)
  assert.equal(validateAssignedPassword('').ok, false)
  assert.equal(validateAssignedPassword('short').ok, false)
  assert.equal(validateAssignedPassword('admin').ok, false)
  assert.equal(validateAssignedPassword('goodpass1').ok, true)
  const en = validateAssignedPassword('admin', { locale: 'en' })
  assert.match(String(en.error), /default password/i)
})

test('applyServerPasswordPolicy overrides min length', () => {
  resetPasswordPolicyRuntime()
  applyServerPasswordPolicy({ min_length: 12, forbidden_defaults: ['admin', 'password'], version: 2 })
  assert.equal(getMinPasswordLength(), 12)
  assert.equal(validateAssignedPassword('goodpass1').ok, false)
  assert.equal(validateAssignedPassword('goodpass1234').ok, true)
  assert.equal(validateAssignedPassword('password').ok, false)
  resetPasswordPolicyRuntime()
  assert.equal(getMinPasswordLength(), 8)
})
