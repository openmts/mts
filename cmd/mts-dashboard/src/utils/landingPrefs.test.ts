import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isAdminOnlyLandingPath,
  loadLandingPath,
  normalizeLandingPath,
  resolveLandingPath,
  saveLandingPath,
} from './landingPrefs.ts'
import { sanitizeRedirect } from './redirect.ts'

function mem() {
  const m = new Map<string, string>()
  return {
    getItem: (k: string) => m.get(k) ?? null,
    setItem: (k: string, v: string) => { m.set(k, v) },
    removeItem: (k: string) => { m.delete(k) },
  }
}

test('normalizeLandingPath known and rejects bad', () => {
  assert.equal(normalizeLandingPath('/query'), '/query')
  assert.equal(normalizeLandingPath('/ops/readiness'), '/ops/readiness')
  assert.equal(normalizeLandingPath('//evil'), '/')
  assert.equal(normalizeLandingPath('/login'), '/')
  assert.equal(normalizeLandingPath('/nope'), '/')
})

test('load/save landing path', () => {
  const s = mem()
  assert.equal(loadLandingPath(s), '/')
  saveLandingPath(s, '/write')
  assert.equal(loadLandingPath(s), '/write')
  saveLandingPath(s, '/')
  assert.equal(loadLandingPath(s), '/')
})

test('resolveLandingPath prefers redirect then pref', () => {
  assert.equal(
    resolveLandingPath({
      redirectRaw: '/audit',
      preferredPath: '/query',
      isAdmin: true,
      sanitizeRedirect,
    }),
    '/audit',
  )
  assert.equal(
    resolveLandingPath({
      preferredPath: '/query',
      isAdmin: true,
      sanitizeRedirect,
    }),
    '/query',
  )
  assert.equal(
    resolveLandingPath({
      preferredPath: '/audit',
      isAdmin: false,
      sanitizeRedirect,
    }),
    '/',
  )
})

test('isAdminOnlyLandingPath', () => {
  assert.equal(isAdminOnlyLandingPath('/audit'), true)
  assert.equal(isAdminOnlyLandingPath('/query'), false)
  assert.equal(isAdminOnlyLandingPath('/databases'), false)
})

test('resolveLandingPath allows databases for non-admin', () => {
  assert.equal(
    resolveLandingPath({
      preferredPath: '/databases',
      isAdmin: false,
      sanitizeRedirect,
    }),
    '/databases',
  )
})
