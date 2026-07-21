import assert from 'node:assert/strict'
import test from 'node:test'
import { copyText } from './clipboard.ts'

test('copyText rejects empty', async () => {
  const r = await copyText('')
  assert.equal(r.ok, false)
  assert.equal(r.method, 'none')
})

test('copyText uses injected writer', async () => {
  let saw = ''
  const r = await copyText('hello-deploy', {
    writeText: async (v) => {
      saw = v
    },
  })
  assert.equal(r.ok, true)
  assert.equal(r.method, 'clipboard')
  assert.equal(saw, 'hello-deploy')
})

test('copyText reports writer failure', async () => {
  const r = await copyText('x', {
    writeText: async () => {
      throw new Error('denied')
    },
  })
  assert.equal(r.ok, false)
  assert.match(String(r.error), /denied/)
})

test('copyText friendly error from Error message', async () => {
  const r = await copyText('x', {
    writeText: async () => {
      throw new Error('no permission')
    },
  })
  assert.equal(r.ok, false)
  assert.equal(r.error, 'no permission')
})
