import assert from 'node:assert/strict'
import test from 'node:test'
import {
  COMMERCIAL_SMOKE_PATHS,
  cspLooksCommercial,
  missingSecurityHeaders,
} from './securityHeaders.ts'

test('missingSecurityHeaders detects absent headers', () => {
  assert.ok(missingSecurityHeaders({}).includes('X-Frame-Options'))
  assert.deepEqual(
    missingSecurityHeaders({
      'X-Content-Type-Options': 'nosniff',
      'X-Frame-Options': 'DENY',
      'Referrer-Policy': 'no-referrer',
      'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
      'Cross-Origin-Opener-Policy': 'same-origin',
    }),
    [],
  )
})

test('cspLooksCommercial', () => {
  assert.equal(cspLooksCommercial(''), false)
  assert.equal(
    cspLooksCommercial("default-src 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self'"),
    true,
  )
})

test('commercial smoke paths cover core surfaces', () => {
  const ids = COMMERCIAL_SMOKE_PATHS.map((p) => p.id)
  for (const need of ['login', 'overview', 'query', 'write', 'databases', 'operations']) {
    assert.ok(ids.includes(need as never), need)
  }
  assert.ok(COMMERCIAL_SMOKE_PATHS.some((p) => p.admin))
  assert.ok(COMMERCIAL_SMOKE_PATHS.some((p) => !p.requiresAuth))
})
