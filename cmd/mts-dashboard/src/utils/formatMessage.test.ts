import assert from 'node:assert/strict'
import test from 'node:test'
import { formatMessage } from './formatMessage.ts'

test('formatMessage replaces placeholders', () => {
  assert.equal(formatMessage('a {user} on {db}', { user: 'alice', db: 'metrics' }), 'a alice on metrics')
  assert.equal(formatMessage('count {count}', { count: 3 }), 'count 3')
  assert.equal(formatMessage('x {missing}', {}), 'x ')
})
