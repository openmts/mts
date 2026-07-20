import assert from 'node:assert/strict'
import test from 'node:test'
import { buildBreadcrumbs } from './breadcrumbs.ts'

test('buildBreadcrumbs root', () => {
  const items = buildBreadcrumbs('/')
  assert.equal(items.length, 1)
  assert.equal(items[0].labelKey, 'overview')
})

test('buildBreadcrumbs nested access grants', () => {
  const items = buildBreadcrumbs('/access/grants')
  assert.ok(items.some((i) => i.path === '/access'))
  assert.equal(items[items.length - 1].path, '/access/grants')
  assert.equal(items[items.length - 1].labelKey, 'accessGrants')
})

test('buildBreadcrumbs readiness', () => {
  const items = buildBreadcrumbs('/ops/readiness')
  assert.equal(items[items.length - 1].labelKey, 'readiness')
})
