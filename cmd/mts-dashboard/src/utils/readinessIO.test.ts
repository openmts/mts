import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyReadinessImport,
  buildReadinessExport,
  parseReadinessImport,
  READINESS_EXPORT_VERSION,
} from './readinessIO.ts'
import { emptyReadinessState } from './readinessState.ts'

test('buildReadinessExport is versioned', () => {
  const payload = buildReadinessExport({
    production: { a: true },
    edgeHttps: { b: true },
    backupSchedule: {},
    updatedAt: '2026-07-20T00:00:00.000Z',
  })
  assert.equal(payload.version, READINESS_EXPORT_VERSION)
  assert.equal(payload.kind, 'mts.readiness')
  assert.equal(payload.state.production.a, true)
  assert.equal(payload.state.edgeHttps.b, true)
})

test('parseReadinessImport accepts wrapped and bare state', () => {
  const wrapped = buildReadinessExport({
    production: { x: true },
    edgeHttps: {},
    backupSchedule: { y: true },
  })
  const p1 = parseReadinessImport(wrapped)
  assert.equal(p1.ok, true)
  if (p1.ok) {
    assert.equal(p1.state.production.x, true)
    assert.equal(p1.state.backupSchedule.y, true)
  }
  const p2 = parseReadinessImport({ production: { z: true }, edgeHttps: {}, backupSchedule: {} })
  assert.equal(p2.ok, true)
  if (p2.ok) assert.equal(p2.state.production.z, true)
})

test('parseReadinessImport rejects bad payloads', () => {
  assert.equal(parseReadinessImport(null).ok, false)
  assert.equal(parseReadinessImport('x').ok, false)
  assert.equal(parseReadinessImport({ kind: 'other', state: {} }).ok, false)
  assert.equal(parseReadinessImport({ kind: 'mts.readiness' }).ok, false)
})

test('applyReadinessImport merge and replace', () => {
  const cur = { production: { a: true }, edgeHttps: { e1: true }, backupSchedule: {} }
  const inc = { production: { b: true }, edgeHttps: {}, backupSchedule: { s1: true } }
  const merged = applyReadinessImport(cur, inc, { merge: true })
  assert.equal(merged.production.a, true)
  assert.equal(merged.production.b, true)
  assert.equal(merged.edgeHttps.e1, true)
  assert.equal(merged.backupSchedule.s1, true)
  const replaced = applyReadinessImport(cur, inc, { merge: false })
  assert.equal(replaced.production.a, undefined)
  assert.equal(replaced.production.b, true)
  assert.deepEqual(replaced.edgeHttps, {})
})

test('empty state export roundtrip parse', () => {
  const p = parseReadinessImport(buildReadinessExport(emptyReadinessState()))
  assert.equal(p.ok, true)
})
