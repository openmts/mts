import assert from 'node:assert/strict'
import test from 'node:test'
import { passwordHintsAllOk, passwordRequirementHints } from './passwordHints.ts'

test('passwordRequirementHints tracks rules', () => {
  const weak = passwordRequirementHints('admin', 'admin', 'admin')
  assert.equal(passwordHintsAllOk(weak), false)
  const strong = passwordRequirementHints('admin', 'AdminChanged!2026', 'AdminChanged!2026')
  assert.equal(passwordHintsAllOk(strong), true)
  assert.ok(strong.every((h) => h.ok))
})
