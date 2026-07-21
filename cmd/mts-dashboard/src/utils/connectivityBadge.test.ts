import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  connectivityBadgeClass,
  connectivityBadgeLabelKey,
  sessionUrgencyBadgeClass,
} from './connectivityBadge.ts'

test('connectivityBadgeClass', () => {
  assert.ok(connectivityBadgeClass('ok').includes('emerald'))
  assert.ok(connectivityBadgeClass('unreachable').includes('red'))
  assert.ok(connectivityBadgeClass('offline').includes('amber'))
  assert.ok(connectivityBadgeClass('x').includes('slate'))
})

test('connectivityBadgeLabelKey', () => {
  assert.equal(connectivityBadgeLabelKey('ok'), 'connectivityOk')
  assert.equal(connectivityBadgeLabelKey('unknown'), 'connectivityUnknown')
})

test('sessionUrgencyBadgeClass', () => {
  assert.ok(sessionUrgencyBadgeClass('critical').includes('red'))
  assert.ok(sessionUrgencyBadgeClass('warn').includes('amber'))
})
