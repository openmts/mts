import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildSessionBadgeMeta } from './sessionBadgeMeta.ts'
import { sessionBadgeTitleText, sessionBadgeAriaText } from './sessionBadgeTitle.ts'

test('buildSessionBadgeMeta without server', () => {
  const m = buildSessionBadgeMeta({
    localLabel: '5m',
    localRemainingMs: 300_000,
    urgency: 'warn',
    serverRemainingSec: null,
    checkedAtMs: null,
    locale: 'zh',
  })
  assert.equal(m.showServerHint, false)
  assert.ok(m.title.includes('5m'))
  assert.equal(m.hint, '')
})

test('buildSessionBadgeMeta warn shows server remaining', () => {
  const m = buildSessionBadgeMeta({
    localLabel: '4m',
    localRemainingMs: 240_000,
    urgency: 'warn',
    serverRemainingSec: 250,
    checkedAtMs: Date.UTC(2026, 6, 22, 10, 0, 0),
    nowMs: Date.UTC(2026, 6, 22, 10, 0, 5),
    locale: 'zh',
  })
  assert.equal(m.showServerHint, true)
  assert.ok(m.title.includes('服务端'))
  assert.ok(m.hint.includes('端'))
  assert.ok(m.serverLabel.length > 0)
})

test('buildSessionBadgeMeta skew threshold', () => {
  const m = buildSessionBadgeMeta({
    localLabel: '10m',
    localRemainingMs: 600_000,
    urgency: 'ok',
    serverRemainingSec: 500,
    checkedAtMs: Date.now(),
    skewThresholdSec: 30,
    locale: 'en',
  })
  assert.equal(m.showServerHint, true)
  assert.ok(m.title.includes('skew'))
  assert.ok(m.hint.includes('Δ'))
})

test('buildSessionBadgeMeta ok without large skew hides hint', () => {
  const m = buildSessionBadgeMeta({
    localLabel: '30m',
    localRemainingMs: 1_800_000,
    urgency: 'ok',
    serverRemainingSec: 1790,
    checkedAtMs: Date.now(),
    skewThresholdSec: 30,
    locale: 'zh',
  })
  assert.equal(m.showServerHint, false)
  assert.ok(m.title.includes('服务端'))
})


test('sessionBadgeTitleText joins', () => {
  assert.equal(sessionBadgeTitleText('hint', 'meta'), 'hint · meta')
  assert.equal(sessionBadgeTitleText('hint', ''), 'hint')
})

test('sessionBadgeAriaText with server', () => {
  const meta = buildSessionBadgeMeta({
    localLabel: '2m',
    localRemainingMs: 120_000,
    urgency: 'critical',
    serverRemainingSec: 100,
    checkedAtMs: Date.now(),
    locale: 'en',
  })
  const aria = sessionBadgeAriaText('Session time left', '2m', 'Unknown', 'Session remaining {local}, server {server}', meta)
  assert.ok(aria.includes('2m'))
  assert.ok(aria.includes('server'))
})
