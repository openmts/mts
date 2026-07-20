import test from 'node:test'
import assert from 'node:assert/strict'
import {
  pickShareLinkButton,
  resolveShareDeepLinkAction,
  stripSensitiveUrlParams,
} from './shareDeepLink.ts'

function makeBtn(opts: {
  testId: string
  disabled?: boolean
  hidden?: boolean
  displayNone?: boolean
}) {
  return {
    getAttribute(name: string) {
      if (name === 'data-testid') return opts.testId
      if (name === 'aria-disabled') return null
      return null
    },
    hasAttribute(name: string) {
      return name === 'disabled' && !!opts.disabled
    },
    hidden: !!opts.hidden,
    style: { display: opts.displayNone ? 'none' : '' },
    offsetParent: opts.displayNone ? null : {},
    getClientRects() {
      return opts.displayNone ? [] : [{ width: 1, height: 1 }]
    },
    click() {
      /* noop */
    },
  }
}

test('stripSensitiveUrlParams removes token-like keys', () => {
  const href = 'http://localhost/query?database=default&token=abc&password=x&foo=1#query-form'
  const out = stripSensitiveUrlParams(href)
  assert.ok(out.includes('database=default'))
  assert.ok(out.includes('foo=1'))
  assert.ok(!out.includes('token='))
  assert.ok(!out.includes('password='))
  assert.ok(out.includes('#query-form'))
})

test('stripSensitiveUrlParams keeps clean urls', () => {
  assert.equal(
    stripSensitiveUrlParams('http://x/query?database=a'),
    'http://x/query?database=a',
  )
})

test('pickShareLinkButton skips disabled and picks first enabled', () => {
  const disabled = makeBtn({ testId: 'query-share-link', disabled: true })
  const enabled = makeBtn({ testId: 'audit-share-link' })
  const root = {
    querySelectorAll() {
      return [disabled, enabled] as unknown as NodeListOf<Element>
    },
  } as unknown as ParentNode
  const picked = pickShareLinkButton(root)
  assert.equal(picked?.getAttribute('data-testid'), 'audit-share-link')
})

test('resolveShareDeepLinkAction prefers share button', () => {
  const enabled = makeBtn({ testId: 'metrics-share-link' })
  const root = {
    querySelectorAll() {
      return [enabled] as unknown as NodeListOf<Element>
    },
  } as unknown as ParentNode
  const r = resolveShareDeepLinkAction({ root, href: 'http://x/' })
  assert.deepEqual(r, { kind: 'clicked', testId: 'metrics-share-link' })
})

test('resolveShareDeepLinkAction falls back to sanitized url', () => {
  const r = resolveShareDeepLinkAction({
    root: null,
    href: 'http://localhost/audit?range=1h&token=x',
  })
  assert.equal(r.kind, 'fallback-url')
  if (r.kind === 'fallback-url') {
    assert.ok(!r.href.includes('token='))
    assert.ok(r.href.includes('range=1h'))
  }
})

test('resolveShareDeepLinkAction empty without href', () => {
  assert.deepEqual(resolveShareDeepLinkAction({ root: null, href: '' }), { kind: 'empty' })
})

test('pickShareLinkButton returns null on empty root', () => {
  assert.equal(pickShareLinkButton(null), null)
})
