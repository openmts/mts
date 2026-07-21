import test from 'node:test'
import assert from 'node:assert/strict'
import {
  pickActionResultRetryButton,
  resolveActionResultRetryAction,
} from './actionResultRetry.ts'

function makeBtn(opts: { disabled?: boolean; hidden?: boolean } = {}) {
  return {
    getAttribute(name: string) {
      if (name === 'data-testid') return 'action-result-retry'
      if (name === 'aria-disabled') return null
      return null
    },
    hasAttribute(name: string) {
      return name === 'disabled' && !!opts.disabled
    },
    hidden: !!opts.hidden,
    style: { display: '' },
    offsetParent: {},
    getClientRects() {
      return [{ width: 1, height: 1 }]
    },
    click() {
      /* noop */
    },
  }
}

test('pickActionResultRetryButton picks first enabled retry', () => {
  const disabled = makeBtn({ disabled: true })
  const enabled = makeBtn()
  const root = {
    querySelectorAll() {
      return [disabled, enabled] as unknown as NodeListOf<Element>
    },
  } as unknown as ParentNode
  assert.equal(pickActionResultRetryButton(root), enabled as unknown as HTMLElement)
})

test('resolveActionResultRetryAction empty without button', () => {
  const root = {
    querySelectorAll() {
      return [] as unknown as NodeListOf<Element>
    },
  } as unknown as ParentNode
  assert.deepEqual(resolveActionResultRetryAction({ root }), { kind: 'empty' })
})

test('resolveActionResultRetryAction clicked when present', () => {
  const enabled = makeBtn()
  const root = {
    querySelectorAll() {
      return [enabled] as unknown as NodeListOf<Element>
    },
  } as unknown as ParentNode
  assert.deepEqual(resolveActionResultRetryAction({ root }), { kind: 'clicked' })
})

test('pickActionResultRetryButton null on empty root', () => {
  assert.equal(pickActionResultRetryButton(null), null)
})
