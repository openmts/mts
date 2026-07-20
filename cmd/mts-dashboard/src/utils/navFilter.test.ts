import assert from 'node:assert/strict'
import test from 'node:test'
import { filterNavItems } from './navFilter.ts'

const items = [
  { to: '/', label: '概览' },
  { to: '/query', label: '查询' },
  { to: '/write', label: '写入' },
  { to: '/ops/readiness', label: '就绪清单' },
]

test('filterNavItems empty query returns all', () => {
  assert.equal(filterNavItems(items, '').length, 4)
  assert.equal(filterNavItems(items, '   ').length, 4)
})

test('filterNavItems matches label and path', () => {
  assert.deepEqual(
    filterNavItems(items, '查').map((x) => x.to),
    ['/query'],
  )
  assert.deepEqual(
    filterNavItems(items, 'write').map((x) => x.to),
    ['/write'],
  )
  assert.deepEqual(
    filterNavItems(items, 'readiness').map((x) => x.to),
    ['/ops/readiness'],
  )
})

test('filterNavItems no match returns empty', () => {
  assert.equal(filterNavItems(items, 'zzz-nope').length, 0)
})
