import assert from 'node:assert/strict'
import test from 'node:test'
import { recentCommandItems } from './commandPalette.ts'

test('recentCommandItems dedupes and caps', () => {
  const items = recentCommandItems(
    [
      { path: '/query', name: 'Query' },
      { path: '/write', name: 'Write' },
      { path: '/query', name: 'Query' },
      { path: '/login', name: 'Login' },
    ],
    2,
  )
  assert.equal(items.length, 2)
  assert.equal(items[0]?.path, '/query')
  assert.equal(items[1]?.path, '/write')
})
