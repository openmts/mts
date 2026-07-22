import assert from 'node:assert/strict'
import test from 'node:test'
import {
  configErrorCodeDeepLink,
  errorActionLinks,
  formatRemediationHint,
  indexErrorCodeContracts,
  isContractRetryable,
  lookupErrorCodeContract,
  normalizeErrorCodeContract,
  remediationPathForCode,
} from './errorCodeContract.ts'

test('normalize and index error code contracts', () => {
  const n = normalizeErrorCodeContract({
    code: 'resource_exhausted',
    http_status: 429,
    grpc_code: 'ResourceExhausted',
    description: 'limit',
    retryable: true,
    category: 'capacity',
    remediation: 'narrow range',
    dashboard_path: '/operations',
  })
  assert.equal(n?.retryable, true)
  assert.equal(n?.category, 'capacity')
  const idx = indexErrorCodeContracts([n, { code: '' }, null])
  assert.equal(idx.size, 1)
  assert.equal(lookupErrorCodeContract('RESOURCE_EXHAUSTED', idx)?.code, 'resource_exhausted')
})

test('deep links and fallback remediation paths', () => {
  assert.equal(configErrorCodeDeepLink('bad_request'), '/config?error_q=bad_request#config-error-codes')
  assert.equal(remediationPathForCode('permission_denied'), '/access-matrix')
  assert.equal(
    remediationPathForCode('x', { code: 'x', http_status: 0, grpc_code: '', description: '', dashboard_path: '/custom' }),
    '/custom',
  )
  const links = errorActionLinks('resource_exhausted', {
    code: 'resource_exhausted',
    http_status: 429,
    grpc_code: 'ResourceExhausted',
    description: 'd',
    dashboard_path: '/operations',
  })
  assert.equal(links.length, 2)
  assert.equal(links[0].path.includes('error_q=resource_exhausted'), true)
  assert.equal(links[1].path, '/operations')
})

test('retryable and remediation hint', () => {
  assert.equal(isContractRetryable('bad_request', { code: 'bad_request', http_status: 400, grpc_code: '', description: '', retryable: false }), false)
  assert.equal(isContractRetryable('internal', null), true)
  assert.equal(isContractRetryable('bad_request', null, true), true)
  assert.match(formatRemediationHint({ code: 'a', http_status: 0, grpc_code: '', description: '', remediation: 'retry' }, 'zh'), /建议/)
  assert.equal(formatRemediationHint({ code: 'a', http_status: 0, grpc_code: '', description: '', remediation: 'retry' }, 'en'), 'retry')
})
