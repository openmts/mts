import assert from 'node:assert/strict'
import test from 'node:test'
import {
  alignUsersMeta,
  buildUsersBatchSummary,
  USERS_BATCH_DISABLED_PATH,
  USERS_LIST_PATH,
} from './usersMetaAlign.ts'

test('alignUsersMeta ok path', () => {
  const a = alignUsersMeta({
    listPath: USERS_LIST_PATH,
    userCount: 5,
    filteredCount: 3,
    adminCount: 1,
    disabledCount: 1,
    selectedCount: 2,
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.path_ok, true)
  assert.equal(a.batch_path, USERS_BATCH_DISABLED_PATH)
  assert.equal(a.user_count, 5)
  assert.equal(a.disabled_count, 1)
})

test('alignUsersMeta all disabled warn', () => {
  const a = alignUsersMeta({ listPath: USERS_LIST_PATH, userCount: 2, disabledCount: 2 })
  assert.equal(a.tone, 'warn')
})

test('buildUsersBatchSummary fail is bad', () => {
  const s = buildUsersBatchSummary({ path: USERS_BATCH_DISABLED_PATH, okCount: 1, failCount: 2 })
  assert.equal(s.tone, 'bad')
  assert.equal(s.fail_count, 2)
})

test('buildUsersBatchSummary cancel/skip warn', () => {
  const s = buildUsersBatchSummary({ okCount: 1, skipCount: 1, cancelled: true })
  assert.equal(s.tone, 'warn')
  assert.equal(s.path, USERS_BATCH_DISABLED_PATH)
})
