import assert from 'node:assert/strict'
import test from 'node:test'
import {
  appendOpsAction,
  buildOpsActionExport,
  loadOpsActionLog,
  saveOpsActionLog,
} from './opsActionLog.ts'

function mem() {
  const data: Record<string, string> = {}
  return {
    getItem: (k: string) => (k in data ? data[k] : null),
    setItem: (k: string, v: string) => {
      data[k] = v
    },
    removeItem: (k: string) => {
      delete data[k]
    },
  }
}

test('appendOpsAction prepends and caps', () => {
  let items = appendOpsAction([], { kind: 'flush', status: 'ok', message: 'a', at: 1 })
  items = appendOpsAction(items, { kind: 'compact', status: 'error', message: 'b', at: 2 })
  assert.equal(items[0].kind, 'compact')
  assert.equal(items.length, 2)
  for (let i = 0; i < 60; i++) {
    items = appendOpsAction(items, { kind: 'other', status: 'ok', message: String(i), at: 100 + i }, 50)
  }
  assert.equal(items.length, 50)
})

test('save/load roundtrip and export', () => {
  const s = mem()
  const items = appendOpsAction([], { kind: 'retention', status: 'ok', message: 'done', at: 9 })
  saveOpsActionLog(items, s)
  const loaded = loadOpsActionLog(s)
  assert.equal(loaded[0].message, 'done')
  const exp = buildOpsActionExport(loaded, '2026-07-20T00:00:00.000Z')
  assert.equal(exp.kind, 'mts.ops.actions')
  assert.equal(exp.items.length, 1)
})
