import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildConfigSchemaExport,
  buildEffectiveConfigExport,
  buildErrorCodesExport,
  formatConfigPretty,
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
    [{ code: 'bad_request', http_status: 400, grpc_code: 'InvalidArgument', description: 'x' }],
    at,
  )
  assert.equal(codes.count, 1)
})

test('formatConfigPretty', () => {
  assert.match(formatConfigPretty({ a: 1 }), /"a": 1/)
  assert.equal(formatConfigPretty(null), '{}')
})
