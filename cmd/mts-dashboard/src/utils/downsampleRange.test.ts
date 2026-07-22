import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildDownsampleRangeBody,
  defaultDownsampleRange,
  formatRunResultMessage,
  rangeActionPath,
  rangeErrorMessage,
  suggestDownsampleRange,
  validateDownsampleRange,
} from './downsampleRange.ts'

test('default and suggest range', () => {
  const def = defaultDownsampleRange(1_700_000_000)
  assert.equal(def.endUnix, 1_700_000_000)
  assert.equal(def.startUnix, 1_700_000_000 - 86400)
  const sug = suggestDownsampleRange(1_699_999_000, 1_700_000_000)
  assert.equal(sug.startUnix, 1_699_999_000 - 3600)
  assert.equal(sug.endUnix, 1_700_000_000)
  assert.deepEqual(suggestDownsampleRange(0, 1000), defaultDownsampleRange(1000))
})

test('validateDownsampleRange rejects bad ranges', () => {
  assert.equal(validateDownsampleRange({ startUnix: 10, endUnix: 20 }).ok, true)
  assert.equal(validateDownsampleRange({ startUnix: 20, endUnix: 10 }).error, 'invalid_range_order')
  assert.equal(validateDownsampleRange({ startUnix: 0, endUnix: 10 }).error, 'invalid_range_non_positive')
  assert.equal(
    validateDownsampleRange({ startUnix: 1, endUnix: 1 + 91 * 86400 }).error,
    'invalid_range_too_wide',
  )
  assert.match(rangeErrorMessage('invalid_range_order', 'zh'), /结束/)
  assert.match(rangeErrorMessage('invalid_range_order', 'en'), /End/)
})

test('buildDownsampleRangeBody snake_case and path', () => {
  const body = buildDownsampleRangeBody({ startUnix: 1, endUnix: 2, advanceWatermark: true })
  assert.deepEqual(body, {
    start_unix: 1,
    end_unix: 2,
    options: { advance_watermark: true },
  })
  const raw = JSON.stringify(body)
  assert.match(raw, /"start_unix":1/)
  assert.match(raw, /"advance_watermark":true/)
  assert.equal(
    rangeActionPath('cpu_1m', 'run-range'),
    '/api/v1/admin/downsample/policies/cpu_1m/run-range',
  )
  assert.match(rangeActionPath('a/b', 'repair'), /a%2Fb/)
})

test('formatRunResultMessage', () => {
  assert.match(formatRunResultMessage('repair', 'p', { windows_processed: 2, points_written: 9 }), /repair p/)
  assert.match(
    formatRunResultMessage('dry-run', 'p', { windows: 3, points_estimate: 4, samples_estimate: 5, estimate_complete: true }),
    /dry-run p/,
  )
})

test('formatRunResultMessage includes optional path', () => {
  assert.match(
    formatRunResultMessage('dry-run', 'p', { windows: 1, points_estimate: 2, samples_estimate: 3, estimate_complete: false }, '/api/v1/admin/downsample/policies/p/dry-run'),
    /dry-run p:.*\(\/api\/v1\/admin\/downsample\/policies\/p\/dry-run\)/,
  )
  assert.match(
    formatRunResultMessage('run', 'p', { windows_processed: 1, points_written: 2 }, '/api/v1/admin/downsample/policies/p/run'),
    /\(\/api\/v1\/admin\/downsample\/policies\/p\/run\)/,
  )
})
