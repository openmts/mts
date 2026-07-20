import assert from 'node:assert/strict'
import test from 'node:test'
import { apiSpecToMarkdown, buildApiSpecExport } from './apiSpecExport.ts'

const sample = {
  version: 'v1',
  namespaces: [
    {
      name: 'admin',
      base_path: '/api/v1/admin',
      endpoints: [{ method: 'GET', path: '/health', auth: 'admin', description: 'h' }],
    },
  ],
}

test('buildApiSpecExport counts', () => {
  const out = buildApiSpecExport(sample)
  assert.equal(out.namespace_count, 1)
  assert.equal(out.endpoint_count, 1)
})

test('apiSpecToMarkdown', () => {
  const md = apiSpecToMarkdown(sample, 'en')
  assert.match(md, /# MTS API Spec/)
  assert.match(md, /GET \/health/)
})
