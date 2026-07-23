import assert from 'node:assert/strict'
import test from 'node:test'
import { alignAccessMatrixMeta } from './accessMatrixMetaAlign.ts'

test('alignAccessMatrixMeta ok', () => {
  const a = alignAccessMatrixMeta({
    totalCount: 10,
    filteredCount: 8,
    withRouteCount: 5,
    adminFullCount: 4,
    userDataCount: 3,
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.filtered_count, 8)
})

test('alignAccessMatrixMeta empty filter warn', () => {
  const a = alignAccessMatrixMeta({ totalCount: 10, filteredCount: 0 })
  assert.equal(a.tone, 'warn')
})
