import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clearSelection,
  filterRowsByIds,
  isAllSelected,
  isSomeSelected,
  resolveExportIds,
  toggleSelectAll,
  toggleSelection,
} from './listSelection.ts'

test('toggleSelection add/remove', () => {
  assert.deepEqual(toggleSelection([], 'a', true), ['a'])
  assert.deepEqual(toggleSelection(['a', 'b'], 'a', false), ['b'])
  assert.deepEqual(toggleSelection(['a'], '', true), ['a'])
})

test('toggleSelectAll keeps non-visible selection', () => {
  assert.deepEqual(toggleSelectAll(['x'], ['a', 'b'], true).sort(), ['a', 'b', 'x'].sort())
  assert.deepEqual(toggleSelectAll(['a', 'x'], ['a', 'b'], false), ['x'])
})

test('all/some selected', () => {
  assert.equal(isAllSelected(['a', 'b'], ['a', 'b']), true)
  assert.equal(isAllSelected(['a'], ['a', 'b']), false)
  assert.equal(isSomeSelected(['a'], ['a', 'b']), true)
  assert.equal(isSomeSelected(['a', 'b'], ['a', 'b']), false)
  assert.equal(isAllSelected([], []), false)
})

test('resolveExportIds and filterRowsByIds', () => {
  assert.deepEqual(resolveExportIds([], ['a', 'b']), ['a', 'b'])
  assert.deepEqual(resolveExportIds(['b', 'z'], ['a', 'b']), ['b'])
  const rows = [{ name: 'a' }, { name: 'b' }, { name: 'c' }]
  assert.deepEqual(
    filterRowsByIds(rows, ['c', 'a'], (r) => r.name).map((r) => r.name),
    ['a', 'c'],
  )
  assert.deepEqual(clearSelection(), [])
})
