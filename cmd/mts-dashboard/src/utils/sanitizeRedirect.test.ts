import test from 'node:test'
import assert from 'node:assert/strict'
import { sanitizeRedirect } from './redirect.ts'

test('sanitizeRedirect accepts internal paths', () => {
  assert.equal(sanitizeRedirect('/query'), '/query')
  assert.equal(sanitizeRedirect('/databases?x=1'), '/databases?x=1')
})

test('sanitizeRedirect rejects open redirects', () => {
  assert.equal(sanitizeRedirect('//evil.com'), null)
  assert.equal(sanitizeRedirect('https://evil.com'), null)
  assert.equal(sanitizeRedirect('/login'), null)
  assert.equal(sanitizeRedirect('login'), null)
})
