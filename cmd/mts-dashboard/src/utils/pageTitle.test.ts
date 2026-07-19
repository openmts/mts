import assert from 'node:assert/strict'
import test from 'node:test'
import { documentTitleForRoute, formatDocumentTitle, resolveRouteTitleKey } from './pageTitle.ts'

test('resolveRouteTitleKey maps known routes', () => {
  assert.equal(resolveRouteTitleKey('Overview'), 'overview')
  assert.equal(resolveRouteTitleKey('Downsample'), 'downsample')
  assert.equal(resolveRouteTitleKey('Unknown'), null)
})

test('formatDocumentTitle joins page and app', () => {
  assert.equal(formatDocumentTitle('查询', 'MTS 控制台'), '查询 · MTS 控制台')
  assert.equal(formatDocumentTitle('', 'MTS 控制台'), 'MTS 控制台')
})

test('documentTitleForRoute uses locale table', () => {
  assert.match(documentTitleForRoute('Readiness', 'zh'), /就绪/)
  assert.match(documentTitleForRoute('Readiness', 'en'), /Readiness/)
  assert.match(documentTitleForRoute('Login', 'en'), /Login|Dashboard/)
})
