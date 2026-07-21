import assert from 'node:assert/strict'
import test from 'node:test'
import { hasQueryResultSnapshot } from './querySnapshot.ts'

test('hasQueryResultSnapshot detects rows/columns/raw/stats', () => {
  assert.equal(hasQueryResultSnapshot({}), false)
  assert.equal(hasQueryResultSnapshot({ rows: 1 }), true)
  assert.equal(hasQueryResultSnapshot({ columns: 2 }), true)
  assert.equal(hasQueryResultSnapshot({ rawOutput: '{}' }), true)
  assert.equal(hasQueryResultSnapshot({ stats: true }), true)
  assert.equal(hasQueryResultSnapshot({ rows: 0, columns: 0, rawOutput: '' }), false)
})
