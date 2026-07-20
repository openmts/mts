import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyNavOrder,
  loadNavOrderPrefs,
  moveNavPath,
  moveNavPathTo,
  parseNavOrderMap,
  resolveSectionOrder,
  saveNavOrderMap,
  setSectionOrder,
} from './navOrder.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
}

test('applyNavOrder reorders and keeps rest', () => {
  const items = [{ to: '/a' }, { to: '/b' }, { to: '/c' }]
  assert.deepEqual(
    applyNavOrder(items, ['/c', '/a']).map((x) => x.to),
    ['/c', '/a', '/b'],
  )
  assert.deepEqual(applyNavOrder(items, []).map((x) => x.to), ['/a', '/b', '/c'])
})

test('moveNavPath up/down bounds', () => {
  assert.deepEqual(moveNavPath(['/a', '/b', '/c'], '/b', 'up'), ['/b', '/a', '/c'])
  assert.deepEqual(moveNavPath(['/a', '/b', '/c'], '/a', 'up'), ['/a', '/b', '/c'])
  assert.deepEqual(moveNavPath(['/a', '/b', '/c'], '/c', 'down'), ['/a', '/b', '/c'])
})

test('load/save nav order prefs', () => {
  const s = mem()
  saveNavOrderMap(s, { workspace: ['/write', '/query'] })
  const loaded = loadNavOrderPrefs(s)
  assert.deepEqual(loaded.workspace, ['/write', '/query'])
  saveNavOrderMap(s, {})
  assert.deepEqual(loadNavOrderPrefs(s), {})
})

test('parseNavOrderMap dedupes', () => {
  const m = parseNavOrderMap({ admin: ['/audit', '/audit', '/config'] })
  assert.deepEqual(m.admin, ['/audit', '/config'])
})

test('resolveSectionOrder and setSectionOrder', () => {
  const paths = ['/query', '/write', '/']
  assert.deepEqual(resolveSectionOrder(paths, ['/write']), ['/write', '/query', '/'])
  const map = setSectionOrder({}, 'workspace', ['/write', '/query'])
  assert.deepEqual(map.workspace, ['/write', '/query'])
})

test('moveNavPathTo drop on target', () => {
  assert.deepEqual(moveNavPathTo(['/a', '/b', '/c'], '/a', '/c'), ['/b', '/c', '/a'])
  assert.deepEqual(moveNavPathTo(['/a', '/b', '/c'], '/c', '/a'), ['/c', '/a', '/b'])
  assert.deepEqual(moveNavPathTo(['/a', '/b', '/c'], '/b', '/b'), ['/a', '/b', '/c'])
  assert.deepEqual(moveNavPathTo(['/a', '/b', '/c'], '/x', '/a'), ['/a', '/b', '/c'])
  assert.deepEqual(moveNavPathTo(['/a', '/b', '/c'], '/a', '/b'), ['/b', '/a', '/c'])
})
