import assert from 'node:assert/strict'
import test from 'node:test'
import {
  appendFieldToken,
  capSeriesList,
  fieldNames,
  seriesLabel,
  tagsToExpr,
} from './seriesMeta.ts'

test('tagsToExpr sorts keys', () => {
  assert.equal(tagsToExpr({ b: '2', a: '1' }), 'a=1,b=2')
  assert.equal(tagsToExpr(null), '')
})

test('seriesLabel prefers tags then id', () => {
  assert.equal(seriesLabel({ tags: { host: 'a' } }), 'host=a')
  assert.equal(seriesLabel({ id: 9 }), '#9')
  assert.equal(seriesLabel({}), '(no tags)')
})

test('capSeriesList truncates', () => {
  const r = capSeriesList([1, 2, 3, 4], 2)
  assert.deepEqual(r.items, [1, 2])
  assert.equal(r.truncated, true)
  assert.equal(r.total, 4)
  assert.equal(capSeriesList([1], 5).truncated, false)
})

test('fieldNames and appendFieldToken', () => {
  assert.deepEqual(fieldNames([{ name: 'a' }, { name: ' ' }, { name: 'b' }]), ['a', 'b'])
  assert.equal(appendFieldToken('usage', 'cpu'), 'usage,cpu')
  assert.equal(appendFieldToken('usage,cpu', 'usage'), 'usage,cpu')
})
