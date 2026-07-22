import assert from 'node:assert/strict'
import test from 'node:test'
import { apiSpecToMarkdown, buildApiSpecExport } from './apiSpecExport.ts'

const sample = {
  version: 'v1',
  namespaces: [
    {
      name: 'admin',
      base_path: '/api/v1/admin',
      endpoints: [
        { method: 'GET', path: '/health', auth: 'admin', description: 'h' },
        {
          method: 'POST',
          path: '/flush',
          auth: 'admin',
          description: 'flush memtables',
          response: 'okResponse{ok,admin_op_busy,last}',
        },
      ],
    },
    {
      name: 'users',
      base_path: '/api/v1/users',
      endpoints: [
        {
          method: 'POST',
          path: '/batch-disabled',
          auth: 'admin',
          description: 'batch disable',
          response: 'batchMutationResponse{ok,ok_count,items,admin_op_busy,last}',
        },
      ],
    },
  ],
}

test('buildApiSpecExport counts', () => {
  const out = buildApiSpecExport(sample)
  assert.equal(out.namespace_count, 2)
  assert.equal(out.endpoint_count, 3)
  assert.equal(out.namespaces[0]?.endpoints[1]?.response?.includes('okResponse'), true)
})

test('apiSpecToMarkdown', () => {
  const md = apiSpecToMarkdown(sample, 'en')
  assert.match(md, /# MTS API Spec/)
  assert.match(md, /GET \/health/)
  assert.match(md, /response=okResponse/)
  assert.match(md, /batchMutationResponse/)
  const zh = apiSpecToMarkdown(sample, 'zh')
  assert.match(zh, /响应=/)
  assert.match(zh, /batchMutationResponse/)
})
