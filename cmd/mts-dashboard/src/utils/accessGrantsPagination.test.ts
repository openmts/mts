import assert from 'node:assert/strict'
import test from 'node:test'
import {
  ACCESS_GRANTS_PAGE_LIMIT,
  accessGrantItemsToBundles,
  advanceAccessGrantsCursor,
  buildAccessGrantsPagePath,
  retreatAccessGrantsCursor,
} from './accessGrantsPagination.ts'

test('access grants pagination builds bounded aggregate paths', () => {
  assert.equal(ACCESS_GRANTS_PAGE_LIMIT, 100)
  assert.equal(
    buildAccessGrantsPagePath(),
    '/api/v1/users/access-grants?limit=100',
  )
  assert.equal(
    buildAccessGrantsPagePath('a/b c', 200),
    '/api/v1/users/access-grants?limit=200&cursor=a%2Fb+c',
  )
  assert.throws(() => buildAccessGrantsPagePath('', 0), /limit/)
  assert.throws(() => buildAccessGrantsPagePath('', 201), /limit/)
})

test('access grants pagination converts complete items including empty grants', () => {
  const bundles = accessGrantItemsToBundles([
    {
      user: { name: 'alice', role: 'admin', disabled: false },
      grants: [{ database: 'metrics', permission: 'read' }],
    },
    {
      user: { name: 'bob', role: 'user', disabled: true },
      grants: [],
    },
  ])

  assert.deepEqual(bundles, [
    {
      user: 'alice',
      role: 'admin',
      disabled: false,
      grants: [{ database: 'metrics', permission: 'read' }],
    },
    {
      user: 'bob',
      role: 'user',
      disabled: true,
      grants: [],
    },
  ])
})

test('access grants pagination advances and retreats cursor history immutably', () => {
  const first = { cursor: '', history: [] as string[] }
  const second = advanceAccessGrantsCursor(first, 'alice')
  const third = advanceAccessGrantsCursor(second, 'carol')

  assert.deepEqual(first, { cursor: '', history: [] })
  assert.deepEqual(second, { cursor: 'alice', history: [''] })
  assert.deepEqual(third, { cursor: 'carol', history: ['', 'alice'] })
  assert.deepEqual(retreatAccessGrantsCursor(third), second)
  assert.deepEqual(retreatAccessGrantsCursor(second), first)
  assert.deepEqual(retreatAccessGrantsCursor(first), first)
})
