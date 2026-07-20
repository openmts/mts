import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildLandingOptionViews,
  filterLandingOptions,
  groupLandingOptions,
} from './landingOptionsView.ts'

test('buildLandingOptionViews', () => {
  const views = buildLandingOptionViews(
    ['/', '/config'],
    (p) => (p === '/' ? 'Overview' : 'Config'),
    (p) => p === '/config',
  )
  assert.equal(views[0].label, 'Overview')
  assert.equal(views[1].adminOnly, true)
})

test('filter and group', () => {
  const items = [
    { path: '/', label: 'Overview', adminOnly: false },
    { path: '/config', label: 'Config', adminOnly: true },
    { path: '/query', label: 'Query', adminOnly: false },
  ]
  assert.equal(filterLandingOptions(items, 'con').length, 1)
  assert.equal(filterLandingOptions(items, '').length, 3)
  const g = groupLandingOptions(items)
  assert.equal(g.common.length, 2)
  assert.equal(g.admin.length, 1)
})
