import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clearDismissedDownsampleHealthFingerprint,
  downsampleHealthFingerprint,
  readDismissedDownsampleHealthFingerprint,
  shouldShowDownsampleHealthBanner,
  writeDismissedDownsampleHealthFingerprint,
  formatDownsampleHealthBannerCopyText,
} from './downsampleHealthBanner.ts'

test('fingerprint stable', () => {
  const a = downsampleHealthFingerprint({ total: 3, error: 1, lagging: 2, max_lag_seconds: 9 })
  const b = downsampleHealthFingerprint({ total: 3, error: 1, lagging: 2, max_lag_seconds: 9 })
  assert.equal(a, b)
  assert.match(a, /^e1:l2:m9:t3$/)
})

test('shouldShow respects error/lag and dismiss fingerprint', () => {
  const summary = { total: 2, error: 1, lagging: 0, max_lag_seconds: 0 }
  assert.equal(
    shouldShowDownsampleHealthBanner({ isAdmin: true, offline: false, summary }),
    true,
  )
  assert.equal(
    shouldShowDownsampleHealthBanner({ isAdmin: true, offline: false, summary: { total: 1 } }),
    false,
  )
  const fp = downsampleHealthFingerprint(summary)
  assert.equal(
    shouldShowDownsampleHealthBanner({
      isAdmin: true,
      offline: false,
      summary,
      dismissedFingerprint: fp,
    }),
    false,
  )
  assert.equal(
    shouldShowDownsampleHealthBanner({
      isAdmin: true,
      offline: false,
      summary: { ...summary, error: 2 },
      dismissedFingerprint: fp,
    }),
    true,
  )
})

test('session storage dismiss roundtrip', () => {
  const store = new Map<string, string>()
  const storage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => {
      store.set(k, v)
    },
    removeItem: (k: string) => {
      store.delete(k)
    },
  }
  writeDismissedDownsampleHealthFingerprint(storage, 'e1:l0:m0:t1')
  assert.equal(readDismissedDownsampleHealthFingerprint(storage), 'e1:l0:m0:t1')
  clearDismissedDownsampleHealthFingerprint(storage)
  assert.equal(readDismissedDownsampleHealthFingerprint(storage), null)
})

test('copy text includes summary line', () => {
  const text = formatDownsampleHealthBannerCopyText({ total: 2, error: 1, lagging: 1, max_lag_seconds: 3 })
  assert.match(text, /MTS downsample health/)
  assert.match(text, /error: 1/)
  assert.match(text, /max_lag_seconds: 3/)
})
