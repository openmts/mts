import assert from 'node:assert/strict'
import test from 'node:test'
import { buildJSONBlob, stampFilename } from './download.ts'

test('stampFilename uses iso-like stamp', () => {
  const name = stampFilename('mts-ops', 'json', new Date('2026-07-20T12:00:00.000Z'))
  assert.match(name, /^mts-ops-2026-07-20T12-00-00\.json$/)
})

test('buildJSONBlob is application/json', async () => {
  const blob = buildJSONBlob({ a: 1 })
  assert.equal(blob.type, 'application/json')
  const text = await blob.text()
  assert.match(text, /"a": 1/)
})
