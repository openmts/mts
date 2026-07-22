import assert from 'node:assert/strict'
import test from 'node:test'
import {
  assignedPasswordHints,
  passwordHintsAllOk,
  passwordHintsProgress,
  passwordRequirementHints,
} from './passwordHints.ts'
import { resetPasswordPolicyRuntime } from './passwordPolicy.ts'

test('passwordRequirementHints tracks rules', () => {
  resetPasswordPolicyRuntime()
  const weak = passwordRequirementHints('admin', 'admin', 'admin')
  assert.equal(passwordHintsAllOk(weak), false)
  const strong = passwordRequirementHints('oldpass12', 'newpass12', 'newpass12')
  assert.equal(passwordHintsAllOk(strong), true)
})

test('assignedPasswordHints', () => {
  resetPasswordPolicyRuntime()
  const weak = assignedPasswordHints('admin')
  assert.equal(passwordHintsAllOk(weak), false)
  const strong = assignedPasswordHints('goodpass1')
  assert.equal(passwordHintsAllOk(strong), true)
  const withConfirm = assignedPasswordHints('goodpass1', 'goodpass1')
  assert.equal(passwordHintsAllOk(withConfirm), true)
  assert.equal(passwordHintsAllOk(assignedPasswordHints('goodpass1', 'other')), false)
})

test('passwordHintsProgress', () => {
  resetPasswordPolicyRuntime()
  const hints = passwordRequirementHints('oldpass12', 'newpass12', 'newpass12')
  const p = passwordHintsProgress(hints)
  assert.equal(p.total, 4)
  assert.equal(p.done, 4)
  assert.equal(p.percent, 100)
  const partial = passwordHintsProgress(passwordRequirementHints('', 'short', 'short'))
  assert.ok(partial.percent < 100)
})
