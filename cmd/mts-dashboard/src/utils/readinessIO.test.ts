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
    deployKit: {},
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
    deployKit: { reviewed: true },
  })
  const p1 = parseReadinessImport(wrapped)
  assert.equal(p1.ok, true)
  if (p1.ok) {
    assert.equal(p1.state.production.x, true)
    assert.equal(p1.state.backupSchedule.y, true)
  }
  const p2 = parseReadinessImport({ production: { z: true }, edgeHttps: {}, backupSchedule: {}, deployKit: {} })
  assert.equal(p2.ok, true)
  if (p2.ok) {
    assert.equal(p2.state.production.z, true)
    assert.deepEqual(p2.state.deployKit, {})
  }
  const p3 = parseReadinessImport({ production: { legacy: true }, edgeHttps: {}, backupSchedule: {} })
  assert.equal(p3.ok, true)
  if (p3.ok) assert.deepEqual(p3.state.deployKit, {})
})

test('parseReadinessImport rejects bad payloads', () => {
  assert.equal(parseReadinessImport(null).ok, false)
  assert.equal(parseReadinessImport('x').ok, false)
  assert.equal(parseReadinessImport({ kind: 'other', state: {} }).ok, false)
  assert.equal(parseReadinessImport({ kind: 'mts.readiness' }).ok, false)
})

test('applyReadinessImport merge and replace', () => {
  const cur = {
    production: { a: true },
    edgeHttps: { e1: true },
    backupSchedule: {},
    deployKit: { reviewed: true },
    signoffNotes: { edgeHttps: 'old' },
  }
  const inc = {
    production: { b: true },
    edgeHttps: {},
    backupSchedule: { s1: true },
    deployKit: { downloaded: true },
    signoffNotes: { backupOffsite: 'remote ok' },
  }
  const merged = applyReadinessImport(cur, inc, { merge: true })
  assert.equal(merged.production.a, true)
  assert.equal(merged.production.b, true)
  assert.equal(merged.edgeHttps.e1, true)
  assert.equal(merged.backupSchedule.s1, true)
  assert.equal(merged.deployKit.reviewed, true)
  assert.equal(merged.deployKit.downloaded, true)
  assert.equal(merged.signoffNotes?.edgeHttps, 'old')
  assert.equal(merged.signoffNotes?.backupOffsite, 'remote ok')
  const replaced = applyReadinessImport(cur, inc, { merge: false })
  assert.equal(replaced.production.a, undefined)
  assert.equal(replaced.production.b, true)
  assert.deepEqual(replaced.edgeHttps, {})
  assert.equal(replaced.deployKit.downloaded, true)
  assert.equal(replaced.deployKit.reviewed, undefined)
  assert.equal(replaced.signoffNotes?.backupOffsite, 'remote ok')
  assert.equal(replaced.signoffNotes?.edgeHttps, undefined)
})

test('empty state export roundtrip parse', () => {
  const p = parseReadinessImport(buildReadinessExport(emptyReadinessState()))
  assert.equal(p.ok, true)
})

test('deployKit export roundtrip', () => {
  const payload = buildReadinessExport({
    production: {},
    edgeHttps: {},
    backupSchedule: {},
    deployKit: { copied: true },
  })
  assert.equal(payload.state.deployKit.copied, true)
  const p = parseReadinessImport(payload)
  assert.equal(p.ok, true)
  if (p.ok) assert.equal(p.state.deployKit.copied, true)
})

test('export includes signoffNotes', () => {
  const payload = buildReadinessExport({
    production: {},
    edgeHttps: {},
    backupSchedule: {},
    deployKit: {},
    signoffNotes: { backupAlert: 'pagerduty' },
  })
  assert.equal(payload.state.signoffNotes?.backupAlert, 'pagerduty')
})
