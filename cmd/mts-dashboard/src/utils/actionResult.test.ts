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

test('actionResultClass and label i18n', () => {
  assert.equal(actionResultClass('ok'), 'mts-alert-ok')
  assert.equal(actionResultClass('error'), 'mts-alert-error')
  assert.equal(actionResultClass('warn'), 'mts-alert-warn')
  assert.equal(actionResultClass('info'), 'mts-alert-info')
  assert.equal(actionResultLabel('ok'), '成功')
  assert.equal(actionResultLabel('ok', 'zh'), '成功')
  assert.equal(actionResultLabel('error', 'zh'), '失败')
  assert.equal(actionResultLabel('warn', 'zh'), '警告')
  assert.equal(actionResultLabel('info', 'zh'), '信息')
  assert.equal(actionResultLabel('ok', 'en'), 'Success')
  assert.equal(actionResultLabel('error', 'en'), 'Failed')
  assert.equal(actionResultLabel('warn', 'en'), 'Warning')
  assert.equal(actionResultLabel('info', 'en'), 'Info')
})
