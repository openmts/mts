import assert from 'node:assert/strict'
import test from 'node:test'
import { nextSessionProbe } from './sessionProbe.ts'

test('nextSessionProbe critical more frequent', () => {
  const d = nextSessionProbe('critical', 0)
  assert.equal(d.shouldProbe, false)
  assert.ok(d.nextDelayMs <= 20_000)
  assert.equal(nextSessionProbe('critical', 25_000).shouldProbe, true)
})

test('nextSessionProbe warn vs ok', () => {
  assert.equal(nextSessionProbe('warn', 61_000).shouldProbe, true)
  assert.equal(nextSessionProbe('ok', 61_000).shouldProbe, false)
  assert.equal(nextSessionProbe('ok', 6 * 60_000).shouldProbe, true)
})

test('nextSessionProbe skips expired', () => {
  assert.equal(nextSessionProbe('expired', 999_999).shouldProbe, false)
})
