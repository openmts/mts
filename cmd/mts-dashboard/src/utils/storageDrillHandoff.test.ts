import assert from 'node:assert/strict'
import test from 'node:test'
import {
  appendStorageDrillEvent,
  emptyStorageDrillHandoff,
  formatStorageDrillHandoffLine,
  parseStorageDrillHandoff,
  recordStorageDrillEvent,
} from './storageDrillHandoff.ts'

test('appendStorageDrillEvent replaces same kind and caps', () => {
  let h = emptyStorageDrillHandoff(new Date('2026-07-23T00:00:00Z'))
  h = appendStorageDrillEvent(h, {
    kind: 'validate', at: 't1', path: '/api/v1/admin/storage/validate', ok: true, summary: 'ok',
  })
  h = appendStorageDrillEvent(h, {
    kind: 'export', at: 't2', path: '/api/v1/admin/storage/export', ok: true, summary: 'exp',
  })
  h = appendStorageDrillEvent(h, {
    kind: 'validate', at: 't3', path: '/api/v1/admin/storage/validate', ok: false, summary: 'bad',
  })
  assert.equal(h.events.length, 2)
  assert.equal(h.events[0].kind, 'validate')
  assert.equal(h.events[0].ok, false)
  assert.equal(h.events[1].kind, 'export')
})

test('parseStorageDrillHandoff rejects bad version', () => {
  assert.equal(parseStorageDrillHandoff({ version: 2, events: [] }), null)
  assert.equal(parseStorageDrillHandoff(null), null)
})

test('recordStorageDrillEvent uses memory storage', () => {
  const mem: Record<string, string> = {}
  const storage = {
    getItem: (k: string) => mem[k] ?? null,
    setItem: (k: string, v: string) => { mem[k] = v },
    removeItem: (k: string) => { delete mem[k] },
    clear: () => { Object.keys(mem).forEach((k) => delete mem[k]) },
    key: () => null,
    length: 0,
  } as Storage
  const h = recordStorageDrillEvent(storage, {
    kind: 'restore-drill',
    at: '2026-07-23T01:00:00Z',
    path: '/api/v1/admin/storage/restore-drill',
    ok: true,
    summary: 'files=2',
  })
  assert.equal(h.events[0].kind, 'restore-drill')
  assert.match(formatStorageDrillHandoffLine(h, 'en'), /restore-drill:ok/)
})
