import assert from 'node:assert/strict'
import test from 'node:test'
import { buildOverviewExport, formatOverviewExportPretty } from './overviewExport.ts'

test('buildOverviewExport', () => {
  const out = buildOverviewExport(
    { connectivity: 'ok', healthy: true, ready: true, readiness_total: 80 },
    new Date('2026-07-20T00:00:00.000Z'),
  )
  assert.equal(out.kind, 'mts.overview.snapshot')
  assert.equal(out.healthy, true)
  assert.equal(out.readiness_total, 80)
})

test('formatOverviewExportPretty', () => {
  const text = formatOverviewExportPretty({ connectivity: 'ok' })
  assert.match(text, /mts.overview.snapshot/)
})
