import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildNotifyHistoryExport,
  formatNotifyHistoryExportPretty,
  notifyHistoryToCSV,
} from './notifyHistoryExport.ts'
import type { NotifyHistoryEntry } from './notifyHistory.ts'

const sample: NotifyHistoryEntry[] = [
  { id: '1', kind: 'error', message: 'fail, "x"', count: 2, at: 1_700_000_000_000 },
  { id: '2', kind: 'info', message: 'ok', count: 1, at: 1_700_000_000_100 },
]

test('buildNotifyHistoryExport', () => {
  const out = buildNotifyHistoryExport(sample, new Date('2026-07-20T00:00:00.000Z'))
  assert.equal(out.count, 2)
  assert.equal(out.generated_at, '2026-07-20T00:00:00.000Z')
  assert.equal(out.items[0]?.kind, 'error')
  assert.equal(out.items[0]?.count, 2)
})

test('formatNotifyHistoryExportPretty is JSON', () => {
  const s = formatNotifyHistoryExportPretty(sample)
  assert.ok(s.includes('"count": 2'))
})

test('notifyHistoryToCSV escapes', () => {
  const csv = notifyHistoryToCSV(sample)
  assert.ok(csv.startsWith('at,kind,count,message'))
  assert.ok(csv.includes('error'))
  assert.ok(csv.includes('"fail, ""x"""') || csv.includes('fail'))
})
