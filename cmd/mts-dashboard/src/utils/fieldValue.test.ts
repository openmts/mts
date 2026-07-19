import test from 'node:test'
import assert from 'node:assert/strict'
import { formatFieldValue, formatFieldsMap } from './fieldValue.ts'

test('formatFieldValue expands FieldValue objects', () => {
  assert.equal(formatFieldValue({ type: 1, float64: 0.7 }), '0.7')
  assert.equal(formatFieldValue({ type: 2, int64: 3 }), '3')
  assert.equal(formatFieldValue({ type: 3, string: 'ok' }), 'ok')
  assert.equal(formatFieldValue({ type: 4, bool: true }), 'true')
  assert.equal(formatFieldValue(1.2), '1.2')
})

test('formatFieldsMap joins keys', () => {
  assert.equal(formatFieldsMap({ usage: { float64: 1 }, n: 2 }), 'usage=1, n=2')
})
