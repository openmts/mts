import assert from 'node:assert/strict'
import test from 'node:test'
import { groupNavItems, sectionIdForPath } from './navSections.ts'

test('sectionIdForPath known', () => {
  assert.equal(sectionIdForPath('/'), 'workspace')
  assert.equal(sectionIdForPath('/audit'), 'admin')
  assert.equal(sectionIdForPath('/account'), 'system')
  assert.equal(sectionIdForPath('/nope'), null)
})

test('groupNavItems order and drop empty', () => {
  const items = [
    { to: '/account', label: 'a' },
    { to: '/query', label: 'q' },
    { to: '/audit', label: 't' },
  ]
  const groups = groupNavItems(items)
  assert.deepEqual(groups.map((g) => g.id), ['workspace', 'admin', 'system'])
  assert.equal(groups[0]?.items[0]?.to, '/query')
  assert.equal(groups[1]?.items[0]?.to, '/audit')
  assert.equal(groups[2]?.items[0]?.to, '/account')
})

test('groupNavItems orphan joins system', () => {
  const groups = groupNavItems([{ to: '/custom', label: 'x' }])
  assert.equal(groups.length, 1)
  assert.equal(groups[0]?.id, 'system')
  assert.equal(groups[0]?.items[0]?.to, '/custom')
})
