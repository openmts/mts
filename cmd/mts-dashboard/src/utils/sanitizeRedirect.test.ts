import test from 'node:test'
import assert from 'node:assert/strict'
import { formatRedirectLabel, sanitizeRedirect, withRedirectQuery } from './redirect.ts'

test('sanitizeRedirect accepts internal paths', () => {
  assert.equal(sanitizeRedirect('/query'), '/query')
  assert.equal(sanitizeRedirect('/databases?x=1'), '/databases?x=1')
  assert.equal(sanitizeRedirect('/ops/readiness#deploy-kit'), '/ops/readiness#deploy-kit')
})

test('sanitizeRedirect rejects open redirects', () => {
  assert.equal(sanitizeRedirect('//evil.com'), null)
  assert.equal(sanitizeRedirect('https://evil.com'), null)
  assert.equal(sanitizeRedirect('/login'), null)
  assert.equal(sanitizeRedirect('/login?x=1'), null)
  assert.equal(sanitizeRedirect('/force-change-password'), null)
  assert.equal(sanitizeRedirect('login'), null)
})

test('formatRedirectLabel truncates', () => {
  assert.equal(formatRedirectLabel('/query'), '/query')
  const long = '/' + 'a'.repeat(120)
  const out = formatRedirectLabel(long, 20)
  assert.ok(out.endsWith('…'))
  assert.ok(out.length <= 20)
})

test('withRedirectQuery merges only safe redirect', () => {
  assert.deepEqual(withRedirectQuery({ reason: 'auth' }, '/query'), {
    reason: 'auth',
    redirect: '/query',
  })
  assert.deepEqual(withRedirectQuery({ reason: 'auth' }, 'https://evil'), {
    reason: 'auth',
  })
})
