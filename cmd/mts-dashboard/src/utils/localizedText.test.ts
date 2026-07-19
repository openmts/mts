import assert from 'node:assert/strict'
import test from 'node:test'
import { textForLocale } from './localizedText.ts'

test('textForLocale picks locale and empty', () => {
  const t = { zh: '中文', en: 'English' }
  assert.equal(textForLocale(t, 'zh'), '中文')
  assert.equal(textForLocale(t, 'en'), 'English')
  assert.equal(textForLocale(undefined, 'en'), '')
})
