import assert from 'node:assert/strict'
import test from 'node:test'
import {
  detailStatCards,
  formatDurationMs,
  isEmptyStats,
  primaryStatCards,
  toneClass,
} from './queryStatsView.ts'

test('formatDurationMs', () => {
  assert.equal(formatDurationMs({ duration_nanos: 1_500_000 }), '1.5ms')
  assert.equal(formatDurationMs({ duration_nanos: 2_500_000_000 }), '2.50s')
})

test('primary and detail cards', () => {
  const s = { shards_scanned: 2, samples_read: 10, samples_returned: 3, duration_nanos: 1e6, errors: 1 }
  assert.equal(primaryStatCards(s).length, 5)
  assert.ok(detailStatCards(s).some((c) => c.key === 'errors' && c.tone === 'rose'))
})

test('isEmptyStats and toneClass', () => {
  assert.equal(isEmptyStats(null), true)
  assert.equal(isEmptyStats({ samples_read: 1 }), false)
  assert.match(toneClass('blue'), /blue/)
})
