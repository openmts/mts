import assert from 'node:assert/strict'
import test from 'node:test'
import {
  actionResultClass,
  actionResultLabel,
  makeActionResult,
} from './actionResult.ts'

test('makeActionResult fills defaults', () => {
  const r = makeActionResult('ok', 'done', 1)
  assert.equal(r.kind, 'ok')
  assert.equal(r.message, 'done')
  assert.equal(r.at, 1)
  assert.equal(makeActionResult('error', '  ').message, '—')
})

test('actionResultClass and label', () => {
  assert.equal(actionResultClass('ok'), 'mts-alert-ok')
  assert.equal(actionResultClass('error'), 'mts-alert-error')
  assert.equal(actionResultClass('warn'), 'mts-alert-warn')
  assert.equal(actionResultClass('info'), 'mts-alert-info')
  assert.equal(actionResultLabel('ok'), '成功')
})
