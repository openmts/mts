import assert from 'node:assert/strict'
import test from 'node:test'
import { recentCommandItems } from './commandPalette.ts'

test('recentCommandItems dedupes and caps', () => {
  const items = recentCommandItems(
    [
      { path: '/query', name: 'Query', at: 1 },
      { path: '/write', name: 'Write', at: 2 },
      { path: '/query', name: 'Query', at: 3 },
      { path: '/login', name: 'Login', at: 4 },
    ],
    2,
  )
  assert.equal(items.length, 2)
  assert.equal(items[0]?.path, '/query')
  assert.equal(items[1]?.path, '/write')
})

test('recentCommandItems pins first', () => {
  const items = recentCommandItems(
    [
      { path: '/query', name: 'Query', at: 10 },
      { path: '/write', name: 'Write', at: 1, pinned: true },
      { path: '/about', name: 'About', at: 5 },
    ],
    3,
  )
  assert.equal(items[0]?.path, '/write')
  assert.equal(items[0]?.pinned, true)
  assert.equal(items[1]?.path, '/query')
})
