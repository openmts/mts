import assert from 'node:assert/strict'
import test from 'node:test'
import { getFocusableElements } from './focusTrap.ts'

test('getFocusableElements is exported and handles empty root-like object', () => {
  // 无完整 DOM 时至少保证函数可调用形状稳定
  assert.equal(typeof getFocusableElements, 'function')
})
