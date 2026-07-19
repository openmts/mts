import assert from 'node:assert/strict'
import test from 'node:test'
import {
  archiveFilenames,
  buildReadinessArchive,
  formatReadinessArchiveMarkdown,
} from './readinessArchive.ts'

test('buildReadinessArchive includes score and checklist', () => {
  const a = buildReadinessArchive({
    operator: 'alice',
    note: 'weekly drill',
    now: '2026-07-20T12:00:00.000Z',
    state: {
      production: { tls: true },
      edgeHttps: { 'tls-terminate': true },
      backupSchedule: { cron: true },
      updatedAt: '2026-07-20T11:00:00.000Z',
    },
    score: {
      checklist: 50,
      edgeHttps: 100,
      backupSchedule: 100,
      doctor: 80,
      total: 82,
      reasons: ['checklist_incomplete'],
    },
    doctor: {
      loaded: true,
      ok: true,
      http_tls_enabled: false,
      warn_count: 1,
      checks: [{ level: 'warn', code: 'http_tls', message: 'tls off' }],
    },
  })
  assert.equal(a.kind, 'mts.readiness.archive')
  assert.equal(a.operator, 'alice')
  assert.equal(a.score.total, 82)
  assert.deepEqual(a.checklist.production, ['tls'])
  assert.deepEqual(a.checklist.edgeHttps, ['tls-terminate'])
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /alice/)
  assert.match(md, /82%/)
  assert.match(md, /http_tls/)
  assert.match(md, /weekly drill/)
})

test('archiveFilenames uses iso-like stamp', () => {
  const names = archiveFilenames(new Date('2026-07-20T12:34:56.000Z'))
  assert.match(names.json, /mts-readiness-archive-2026-07-20T12-34-56\.json/)
  assert.match(names.md, /\.md$/)
})
