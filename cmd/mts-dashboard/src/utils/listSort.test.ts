import assert from 'node:assert/strict'
import test from 'node:test'
import {
  compareStrings,
  ariaSortValue,
  cycleSortState,
  emptySortState,
  loadSortState,
  parseSortState,
  saveSortState,
  sortByAccessor,
} from './listSort.ts'

test('cycleSortState asc->desc->clear', () => {
  let s = emptySortState<'name' | 'role'>()
  s = cycleSortState(s, 'name')
  assert.deepEqual(s, { key: 'name', dir: 'asc' })
  s = cycleSortState(s, 'name')
  assert.deepEqual(s, { key: 'name', dir: 'desc' })
  s = cycleSortState(s, 'name')
  assert.deepEqual(s, { key: '', dir: 'asc' })
  s = cycleSortState(s, 'role')
  assert.deepEqual(s, { key: 'role', dir: 'asc' })
})

test('sortByAccessor stable', () => {
  const rows = [
    { name: 'b', role: 'user', disabled: false },
    { name: 'a', role: 'admin', disabled: true },
    { name: 'c', role: 'user', disabled: false },
  ]
  const byName = sortByAccessor(rows, { key: 'name', dir: 'asc' }, {
    name: (r) => r.name,
    role: (r) => r.role,
    status: (r) => r.disabled,
  })
  assert.deepEqual(byName.map((r) => r.name), ['a', 'b', 'c'])
  const byStatus = sortByAccessor(rows, { key: 'status', dir: 'desc' }, {
    name: (r) => r.name,
    role: (r) => r.role,
    status: (r) => r.disabled,
  })
  assert.equal(byStatus[0]!.disabled, true)
  assert.equal(sortByAccessor(rows, emptySortState(), { name: (r) => r.name }).length, 3)
  assert.ok(compareStrings('a2', 'a10') < 0)
})

test('parse/load/save sort prefs', () => {
  const allowed = ['name', 'role'] as const
  assert.deepEqual(parseSortState({ key: 'role', dir: 'desc' }, allowed), {
    key: 'role',
    dir: 'desc',
  })
  assert.deepEqual(parseSortState({ key: 'nope' }, allowed), emptySortState())
  const m = new Map<string, string>()
  const storage = {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
  saveSortState(storage, 'k', { key: 'name', dir: 'asc' })
  assert.deepEqual(loadSortState(storage, 'k', allowed), { key: 'name', dir: 'asc' })
  saveSortState(storage, 'k', emptySortState())
  assert.deepEqual(loadSortState(storage, 'k', allowed), emptySortState())
})

test('ariaSortValue maps sort state', () => {
  assert.equal(ariaSortValue(emptySortState(), 'name'), 'none')
  assert.equal(ariaSortValue({ key: 'name', dir: 'asc' }, 'name'), 'ascending')
  assert.equal(ariaSortValue({ key: 'name', dir: 'desc' }, 'name'), 'descending')
  assert.equal(ariaSortValue({ key: 'name', dir: 'asc' }, 'role'), 'none')
})

