import assert from 'node:assert/strict'
import test from 'node:test'

test('分享 URL 在根路径和子路径部署时保留正确应用前缀', async () => {
  let module: typeof import('./shareURL.ts')
  try {
    module = await import('./shareURL.ts')
  } catch {
    assert.fail('BASE_URL 感知的分享 URL helper 尚未实现')
  }

  const path = '/query?database=default#query-form'
  assert.equal(
    module.buildAbsoluteAppURL('https://example.test', '/', path),
    'https://example.test/query?database=default#query-form',
  )
  assert.equal(
    module.buildAbsoluteAppURL('https://example.test', '/mts/', path),
    'https://example.test/mts/query?database=default#query-form',
  )
})

test('分享 URL 不重复已有的子路径前缀', async () => {
  let module: typeof import('./shareURL.ts')
  try {
    module = await import('./shareURL.ts')
  } catch {
    assert.fail('BASE_URL 感知的分享 URL helper 尚未实现')
  }

  assert.equal(
    module.buildAbsoluteAppURL('https://example.test', '/mts/', '/mts/audit?q=write#audit-filters'),
    'https://example.test/mts/audit?q=write#audit-filters',
  )
  assert.equal(
    module.buildAbsoluteAppURL('https://example.test', '/mts/', '/mts?q=write#top'),
    'https://example.test/mts?q=write#top',
  )
})
