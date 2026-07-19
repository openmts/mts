import assert from 'node:assert/strict'
import test from 'node:test'
import {
  loadRecentRoutes,
  pushRecentRoute,
  recordRecentRoute,
  saveRecentRoutes,
} from './recentRoutes.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
  }
}

test('pushRecentRoute dedupes and caps', () => {
  let items = pushRecentRoute([], { path: '/query', name: 'Query', at: 1 })
  items = pushRecentRoute(items, { path: '/write', name: 'Write', at: 2 })
  items = pushRecentRoute(items, { path: '/query', name: 'Query', at: 3 })
  assert.equal(items[0]?.path, '/query')
  assert.equal(items.length, 2)
  assert.equal(normalizeLoginSkipped(), true)
})

function normalizeLoginSkipped() {
  const items = pushRecentRoute([], { path: '/login', name: 'Login', at: 1 })
  return items.length === 0
}

test('recordRecentRoute roundtrip storage', () => {
  const s = mem()
  recordRecentRoute('/ops/readiness', 'Readiness', s)
  recordRecentRoute('/downsample', 'Downsample', s)
  const loaded = loadRecentRoutes(s)
  assert.equal(loaded[0]?.path, '/downsample')
  assert.equal(loaded[1]?.path, '/ops/readiness')
  saveRecentRoutes([], s)
  assert.equal(loadRecentRoutes(s).length, 0)
})
