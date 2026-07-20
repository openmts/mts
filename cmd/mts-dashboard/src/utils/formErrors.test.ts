import assert from 'node:assert/strict'
import test from 'node:test'
import { makeFormErrorT } from './formErrors.ts'

test('makeFormErrorT formats and falls back to key', () => {
  const t = makeFormErrorT({ hello: 'Hi {name}' })
  assert.equal(t('hello', { name: 'A' }), 'Hi A')
  assert.equal(t('missing'), 'missing')
})
