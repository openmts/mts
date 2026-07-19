import assert from 'node:assert/strict'
import test from 'node:test'
import {
  emptySessionGuardState,
  nextSessionGuardAction,
  shouldShowSessionBadge,
} from './sessionGuard.ts'

test('nextSessionGuardAction toasts warn once then none', () => {
  const now = 1_000_000
  const exp = now + 5 * 60_000
  let state = emptySessionGuardState()
  let r = nextSessionGuardAction(exp, now, state)
  assert.equal(r.action.type, 'toast')
  if (r.action.type === 'toast') assert.equal(r.action.urgency, 'warn')
  state = r.state
  r = nextSessionGuardAction(exp, now, state)
  assert.equal(r.action.type, 'none')
})

test('nextSessionGuardAction critical skips duplicate and covers warn', () => {
  const now = 1_000_000
  const exp = now + 60_000
  let state = emptySessionGuardState()
  let r = nextSessionGuardAction(exp, now, state)
  assert.equal(r.action.type, 'toast')
  if (r.action.type === 'toast') assert.equal(r.action.urgency, 'critical')
  state = r.state
  r = nextSessionGuardAction(exp, now, state)
  assert.equal(r.action.type, 'none')
})

test('nextSessionGuardAction expire once', () => {
  const now = 1_000_000
  let state = emptySessionGuardState()
  let r = nextSessionGuardAction(now - 1, now, state)
  assert.equal(r.action.type, 'expire')
  state = r.state
  r = nextSessionGuardAction(now - 1, now, state)
  assert.equal(r.action.type, 'none')
})

test('shouldShowSessionBadge includes ok', () => {
  assert.equal(shouldShowSessionBadge('ok'), true)
  assert.equal(shouldShowSessionBadge('unknown'), false)
})
