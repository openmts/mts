import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildConfigSchemaExport,
  buildEffectiveConfigExport,
  buildErrorCodesExport,
  formatConfigPretty,
  summarizeEffectiveConfig,
} from './configExport.ts'

const at = new Date('2026-07-20T12:00:00.000Z')

test('buildEffectiveConfigExport', () => {
  const out = buildEffectiveConfigExport({ a: 1 }, at)
  assert.equal(out.kind, 'mts.config.effective')
  assert.equal(out.config.a, 1)
  assert.equal(out.generated_at, '2026-07-20T12:00:00.000Z')
})

test('buildConfigSchemaExport and error codes', () => {
  const schema = buildConfigSchemaExport([{ name: 'x', description: 'd' }], at)
  assert.equal(schema.count, 1)
  const codes = buildErrorCodesExport(
    [{
      code: 'bad_request',
      http_status: 400,
      grpc_code: 'InvalidArgument',
      description: 'x',
      retryable: false,
      category: 'client',
      remediation: 'check params',
      dashboard_path: '/config?error_q=bad_request#config-error-codes',
    }],
    at,
  )
  assert.equal(codes.count, 1)
  assert.equal(codes.codes[0].category, 'client')
  assert.equal(codes.codes[0].retryable, false)
})

test('formatConfigPretty', () => {
  assert.match(formatConfigPretty({ a: 1 }), /"a": 1/)
  assert.equal(formatConfigPretty(null), '{}')
})

test('summarizeEffectiveConfig counts sections and sensitive keys', () => {
  const s = summarizeEffectiveConfig({
    http: { addr: '127.0.0.1:8086' },
    auth: { token_ttl: '1h', admin_password: 'x' },
    limits: { max_write_points: 1000 },
  }, '/api/v1/admin/config/effective')
  assert.ok(s)
  assert.equal(s!.top_level_keys, 3)
  assert.deepEqual(s!.sections, ['auth', 'http', 'limits'])
  assert.ok(s!.leaf_count >= 3)
  assert.ok(s!.sensitive_key_hits >= 1)
  assert.equal(s!.path, '/api/v1/admin/config/effective')
})

test('summarizeEffectiveConfig returns null for empty', () => {
  assert.equal(summarizeEffectiveConfig(null), null)
  assert.equal(summarizeEffectiveConfig(undefined), null)
})
