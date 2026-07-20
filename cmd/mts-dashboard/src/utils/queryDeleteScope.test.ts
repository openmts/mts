import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildQueryDeleteScope,
  formatQueryDeleteScopeMessage,
  tagsToShortExpr,
} from './queryDeleteScope.ts'

test('tagsToShortExpr sorts', () => {
  assert.equal(tagsToShortExpr({ b: 2, a: 1 }), 'a=1,b=2')
  assert.equal(tagsToShortExpr(null), '')
})

test('buildQueryDeleteScope defaults', () => {
  const s = buildQueryDeleteScope({ database: 'db', measurement: 'cpu' })
  assert.equal(s.database, 'db')
  assert.equal(s.measurement, 'cpu')
  assert.equal(s.retention_policy, 'autogen')
  assert.equal(s.hasTimeBound, false)
})

test('formatQueryDeleteScopeMessage includes warn', () => {
  const s = buildQueryDeleteScope({ database: 'db' })
  const msg = formatQueryDeleteScopeMessage(s, {
    database: 'DB',
    retention: 'RP',
    measurement: 'M',
    tags: 'T',
    start: 'S',
    end: 'E',
    noTags: '(none)',
    warnNoTime: 'WARN',
  })
  assert.match(msg, /DB: db/)
  assert.match(msg, /WARN/)
})
