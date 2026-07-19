import assert from 'node:assert/strict'
import test from 'node:test'
import {
  hashTargetId,
  scheduleScrollToHash,
  scrollToHashTarget,
} from './hashScroll.ts'

test('hashTargetId strips # and decodes', () => {
  assert.equal(hashTargetId(''), '')
  assert.equal(hashTargetId('#'), '')
  assert.equal(hashTargetId('#deploy-kit'), 'deploy-kit')
  assert.equal(hashTargetId('signoff-notes'), 'signoff-notes')
  assert.equal(hashTargetId('#data%2Drestore'), 'data-restore')
})

test('scrollToHashTarget scrolls when element exists', () => {
  let scrolled = false
  const root = {
    getElementById: (id: string) =>
      id === 'deploy-kit'
        ? {
            scrollIntoView: () => {
              scrolled = true
            },
          }
        : null,
  }
  assert.equal(scrollToHashTarget('#deploy-kit', root), true)
  assert.equal(scrolled, true)
  assert.equal(scrollToHashTarget('#missing', root), false)
  assert.equal(scrollToHashTarget('#deploy-kit', null), false)
})

test('scheduleScrollToHash uses scheduler', () => {
  let scrolled = false
  const root = {
    getElementById: () => ({
      scrollIntoView: () => {
        scrolled = true
      },
    }),
  }
  let ran = false
  scheduleScrollToHash('#x', root, (cb) => {
    ran = true
    cb()
  })
  assert.equal(ran, true)
  assert.equal(scrolled, true)
})
