import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_BACK_TO_TOP_THRESHOLD,
  scrollElementToTop,
  shouldShowBackToTop,
} from './scrollTop.ts'

test('shouldShowBackToTop threshold', () => {
  assert.equal(shouldShowBackToTop(0), false)
  assert.equal(shouldShowBackToTop(DEFAULT_BACK_TO_TOP_THRESHOLD - 1), false)
  assert.equal(shouldShowBackToTop(DEFAULT_BACK_TO_TOP_THRESHOLD), true)
  assert.equal(shouldShowBackToTop(999, 100), true)
  assert.equal(shouldShowBackToTop(Number.NaN), false)
})

test('scrollElementToTop uses scrollTo then fallback', () => {
  let called = false
  const ok = scrollElementToTop({
    scrollTop: 100,
    scrollTo(opts) {
      called = true
      assert.equal(opts.top, 0)
    },
  })
  assert.equal(ok, true)
  assert.equal(called, true)

  const el = { scrollTop: 50 }
  assert.equal(scrollElementToTop(el, 'auto'), true)
  assert.equal(el.scrollTop, 0)
  assert.equal(scrollElementToTop(null), false)
})
