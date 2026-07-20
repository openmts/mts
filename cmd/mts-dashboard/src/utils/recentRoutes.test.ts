import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clearRecentRoutes,
  loadRecentRoutes,
  pushRecentRoute,
  recordRecentRoute,
  saveRecentRoutes,
  setRecentRoutePinned,
  sortRecentRoutes,
} from './recentRoutes.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
}

test('pushRecentRoute dedupes and caps', () => {
  let items = pushRecentRoute([], { path: '/query', name: 'Query', at: 1 })
  items = pushRecentRoute(items, { path: '/write', name: 'Write', at: 2 })
  items = pushRecentRoute(items, { path: '/query', name: 'Query', at: 3 })
  assert.equal(items[0]?.path, '/query')
  assert.equal(items.length, 2)
  assert.equal(pushRecentRoute([], { path: '/login', name: 'Login', at: 1 }).length, 0)
})

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

test('pin survives re-visit and sorts first', () => {
  const s = mem()
  recordRecentRoute('/query', 'Query', s)
  recordRecentRoute('/write', 'Write', s)
  setRecentRoutePinned('/query', true, s)
  recordRecentRoute('/write', 'Write', s)
  const items = loadRecentRoutes(s)
  assert.equal(items[0]?.path, '/query')
  assert.equal(items[0]?.pinned, true)
})

test('clearRecentRoutes keeps pinned by default', () => {
  const s = mem()
  recordRecentRoute('/query', 'Query', s)
  recordRecentRoute('/write', 'Write', s)
  setRecentRoutePinned('/query', true, s)
  const kept = clearRecentRoutes(s)
  assert.equal(kept.length, 1)
  assert.equal(kept[0]?.path, '/query')
  assert.equal(clearRecentRoutes(s, { all: true }).length, 0)
})

test('sortRecentRoutes pins first', () => {
  const items = sortRecentRoutes([
    { path: '/a', name: '', at: 2 },
    { path: '/b', name: '', at: 3, pinned: true },
    { path: '/c', name: '', at: 1, pinned: true },
  ])
  assert.equal(items[0]?.path, '/b')
  assert.equal(items[1]?.path, '/c')
  assert.equal(items[2]?.path, '/a')
})
