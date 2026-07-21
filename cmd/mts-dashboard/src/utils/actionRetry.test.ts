import assert from 'node:assert/strict'
import test from 'node:test'
import { createActionRetry } from './actionRetry.ts'

test('createActionRetry report enables retry; setActionError does not', () => {
  const msgs: string[] = []
  const api = createActionRetry<'create' | 'delete'>({
    notifyError: (m) => { msgs.push(m) },
  })
  api.reportActionError('create', new Error('boom'))
  assert.equal(api.lastFailedAction.value, 'create')
  assert.equal(api.actionResult.value?.kind, 'error')
  assert.equal(api.canRetryAction.value, true)
  assert.equal(msgs.length, 1)

  api.setActionError('offline')
  assert.equal(api.lastFailedAction.value, null)
  assert.equal(api.canRetryAction.value, false)

  api.reportActionError('delete', new Error('x'), { name: 'u1' })
  assert.equal(api.actionContext.value.name, 'u1')
  api.setActionOk('done')
  assert.equal(api.lastFailedAction.value, null)
  assert.equal(api.actionResult.value?.kind, 'ok')
  assert.equal(api.canRetryAction.value, false)

  api.clearActionResult()
  assert.equal(api.actionResult.value, null)
})
