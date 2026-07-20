import assert from 'node:assert/strict'
import test from 'node:test'
import { healthStatusLabel, healthStatusToneClass } from './healthStatusLabel.ts'

test('healthStatusLabel maps known and passthrough', () => {
  assert.equal(healthStatusLabel('ok', 'zh'), '正常')
  assert.equal(healthStatusLabel('OK', 'en'), 'OK')
  assert.equal(healthStatusLabel('failed', 'zh'), '失败')
  assert.equal(healthStatusLabel('custom_x', 'en'), 'custom_x')
  assert.equal(healthStatusLabel('', 'zh'), '—')
})

test('healthStatusToneClass', () => {
  assert.match(healthStatusToneClass('passed'), /green/)
  assert.match(healthStatusToneClass('warn'), /amber/)
  assert.match(healthStatusToneClass('error'), /red/)
})
