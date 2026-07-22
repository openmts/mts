import assert from 'node:assert/strict'
import test from 'node:test'
import {
  formatStorageBytes,
  normalizeDataSnapshotResult,
  normalizeRestoreDrillResult,
} from './storageResultView.ts'

test('normalizeDataSnapshotResult defaults', () => {
  const v = normalizeDataSnapshotResult({ ok: true, path: '/p', files: 3, bytes: 1024 })
  assert.equal(v?.ok, true)
  assert.equal(v?.path, '/p')
  assert.equal(v?.files, 3)
  assert.equal(v?.bytes, 1024)
})

test('normalizeRestoreDrillResult tone', () => {
  const ok = normalizeRestoreDrillResult({
    ok: true,
    path: '/api/v1/admin/storage/restore-drill',
    source: '/s',
    target: '/t',
    files: 2,
    bytes: 10,
    check_issues: 0,
    check_fatals: 0,
  })
  assert.equal(ok?.tone, 'ok')
  assert.equal(ok?.ok, true)

  const warn = normalizeRestoreDrillResult({
    ok: true,
    source: '/s',
    target: '/t',
    check_issues: 2,
    check_fatals: 0,
  })
  assert.equal(warn?.tone, 'warn')

  const bad = normalizeRestoreDrillResult({
    ok: true,
    source: '/s',
    target: '/t',
    check_issues: 1,
    check_fatals: 1,
  })
  assert.equal(bad?.tone, 'bad')
  assert.equal(bad?.ok, false)
  assert.equal(bad?.path, '/api/v1/admin/storage/restore-drill')
})

test('formatStorageBytes', () => {
  assert.equal(formatStorageBytes(512), '512 B')
  assert.match(formatStorageBytes(2048), /KB/)
})
