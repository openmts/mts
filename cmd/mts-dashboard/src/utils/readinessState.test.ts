import assert from 'node:assert/strict'
import test from 'node:test'
import {
  completedIds,
  emptyReadinessState,
  loadReadinessState,
  READINESS_STORAGE_KEY,
  saveReadinessState,
  setReadinessFlag,
  setSignoffNote,
} from './readinessState.ts'

function memStorage(seed: Record<string, string> = {}) {
  const data = { ...seed }
  return {
    getItem: (k: string) => (k in data ? data[k] : null),
    setItem: (k: string, v: string) => {
      data[k] = v
    },
  }
}

test('load empty and save roundtrip', () => {
  const s = memStorage()
  assert.deepEqual(loadReadinessState(s), emptyReadinessState())
  const saved = saveReadinessState(
    { production: { a: true }, edgeHttps: {}, backupSchedule: { b: true }, deployKit: { reviewed: true } },
    s,
  )
  assert.ok(saved.updatedAt)
  const loaded = loadReadinessState(s)
  assert.equal(loaded.production.a, true)
  assert.equal(loaded.backupSchedule.b, true)
  assert.equal(loaded.deployKit.reviewed, true)
})

test('setReadinessFlag toggles and completedIds', () => {
  const s = memStorage()
  setReadinessFlag('edgeHttps', 'tls-terminate', true, s)
  setReadinessFlag('edgeHttps', 'hsts-header', true, s)
  let st = loadReadinessState(s)
  assert.deepEqual(completedIds(st.edgeHttps).sort(), ['hsts-header', 'tls-terminate'])
  setReadinessFlag('edgeHttps', 'tls-terminate', false, s)
  st = loadReadinessState(s)
  assert.deepEqual(completedIds(st.edgeHttps), ['hsts-header'])
})

test('deployKit local hints are independent of score sections', () => {
  const s = memStorage()
  setReadinessFlag('deployKit', 'reviewed', true, s)
  setReadinessFlag('deployKit', 'downloaded', true, s)
  const st = loadReadinessState(s)
  assert.deepEqual(completedIds(st.deployKit).sort(), ['downloaded', 'reviewed'])
  assert.deepEqual(completedIds(st.production), [])
})

test('loadReadinessState tolerates bad json and legacy without deployKit', () => {
  const s = memStorage({ 'mts.dashboard.readiness.v1': '{bad' })
  assert.deepEqual(loadReadinessState(s), emptyReadinessState())
  const legacy = memStorage({
    'mts.dashboard.readiness.v1': JSON.stringify({
      production: { 'https-edge': true },
      edgeHttps: {},
      backupSchedule: {},
    }),
  })
  const st = loadReadinessState(legacy)
  assert.equal(st.production['https-edge'], true)
  assert.deepEqual(st.deployKit, {})
})

test('signoff notes persist and trim', () => {
  const s = memStorage()
  setSignoffNote('edgeHttps', '  cert ok by alice 2026-07-20  ', s)
  setSignoffNote('backupOffsite', 'rsync to backup-host keep=7', s)
  const st = loadReadinessState(s)
  assert.equal(st.signoffNotes?.edgeHttps, 'cert ok by alice 2026-07-20')
  assert.equal(st.signoffNotes?.backupOffsite, 'rsync to backup-host keep=7')
  setSignoffNote('edgeHttps', '   ', s)
  assert.equal(loadReadinessState(s).signoffNotes?.edgeHttps, undefined)
})

test('legacy state without signoffNotes loads empty notes', () => {
  const legacy = memStorage({
    [READINESS_STORAGE_KEY]: JSON.stringify({
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: { reviewed: true },
    }),
  })
  const st = loadReadinessState(legacy)
  assert.deepEqual(st.signoffNotes, {})
  assert.equal(st.deployKit.reviewed, true)
})
