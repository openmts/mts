import assert from 'node:assert/strict'
import test from 'node:test'
import { filterSeriesListLocal, seriesMatchesLocal } from './seriesFilter.ts'

const sample = [
  { id: 1, measurement: 'cpu', tags: { host: 'h0', zone: 'z1' } },
  { id: 2, measurement: 'mem', tags: { host: 'h1' } },
]

test('seriesMatchesLocal empty query matches all', () => {
  assert.equal(seriesMatchesLocal(sample[0], ''), true)
  assert.equal(seriesMatchesLocal(sample[0], '   '), true)
})

test('seriesMatchesLocal by host tag and measurement', () => {
  assert.equal(seriesMatchesLocal(sample[0], 'h0'), true)
  assert.equal(seriesMatchesLocal(sample[0], 'cpu'), true)
  assert.equal(seriesMatchesLocal(sample[0], 'h1'), false)
})

test('filterSeriesListLocal filters list', () => {
  assert.equal(filterSeriesListLocal(sample, 'h1').length, 1)
  assert.equal(filterSeriesListLocal(sample, 'h1')[0].id, 2)
  assert.equal(filterSeriesListLocal(sample, '').length, 2)
})
