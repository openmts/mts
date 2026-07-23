import assert from 'node:assert/strict'
import test from 'node:test'
import {
  alignAccessGrantsMeta,
  preferredPermissionsPath,
  USERS_LIST_PATH,
} from './accessGrantsMetaAlign.ts'

test('preferredPermissionsPath', () => {
  assert.equal(preferredPermissionsPath('alice'), '/api/v1/users/alice/database-permissions')
  assert.match(preferredPermissionsPath(''), /\{name\}/)
})

test('alignAccessGrantsMeta ok', () => {
  const a = alignAccessGrantsMeta({
    usersListPath: USERS_LIST_PATH,
    permissionsPathSample: '/api/v1/users/alice/database-permissions',
    grantCount: 3,
    filteredCount: 2,
    userCount: 2,
    databaseCount: 1,
  })
  assert.equal(a.tone, 'ok')
  assert.equal(a.path_ok, true)
  assert.equal(a.grant_count, 3)
})

test('alignAccessGrantsMeta partial is warn', () => {
  const a = alignAccessGrantsMeta({
    usersListPath: USERS_LIST_PATH,
    permissionsPathSample: '/api/v1/users/a/database-permissions',
    partialErrorCount: 1,
    grantCount: 1,
  })
  assert.equal(a.tone, 'warn')
})

test('alignAccessGrantsMeta bad path', () => {
  const a = alignAccessGrantsMeta({ usersListPath: '/x', permissionsPathSample: '/y' })
  assert.equal(a.tone, 'bad')
  assert.equal(a.path_ok, false)
})
