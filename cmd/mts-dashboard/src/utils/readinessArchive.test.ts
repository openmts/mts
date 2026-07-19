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
      production: { 'https-edge': true },
      edgeHttps: { 'tls-terminate': true },
      backupSchedule: { 'cron-schedule': true },
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
  assert.equal(a.locale, 'zh')
  assert.equal(a.operator, 'alice')
  assert.equal(a.score.total, 82)
  assert.deepEqual(a.checklist.production, ['https-edge'])
  assert.deepEqual(a.checklist.edgeHttps, ['tls-terminate'])
  assert.ok(a.catalog.production.some((x) => x.id === 'https-edge' && x.done && /边缘|HTTPS/.test(x.title)))
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /alice/)
  assert.match(md, /82%/)
  assert.match(md, /http_tls/)
  assert.match(md, /weekly drill/)
  assert.match(md, /边缘 HTTPS \/ TLS|Edge HTTPS/)
})

test('buildReadinessArchive english catalog titles', () => {
  const a = buildReadinessArchive({
    locale: 'en',
    operator: 'bob',
    now: '2026-07-20T12:00:00.000Z',
    state: {
      production: { 'https-edge': true },
      edgeHttps: { 'tls-terminate': true },
      backupSchedule: {},
      updatedAt: '2026-07-20T11:00:00.000Z',
    },
    score: {
      checklist: 10,
      edgeHttps: 20,
      backupSchedule: 30,
      doctor: 40,
      total: 25,
      reasons: [],
    },
    doctor: { loaded: false },
  })
  assert.equal(a.locale, 'en')
  const edge = a.catalog.production.find((x) => x.id === 'https-edge')
  assert.equal(edge?.title, 'Edge HTTPS / TLS')
  assert.equal(a.catalog.edgeHttps.find((x) => x.id === 'tls-terminate')?.title, 'Edge TLS termination')
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /commercial readiness drill archive/i)
  assert.match(md, /Edge HTTPS \/ TLS/)
  assert.match(md, /does not prove production human acceptance/i)
})

test('archiveFilenames uses iso-like stamp', () => {
  const names = archiveFilenames(new Date('2026-07-20T12:34:56.000Z'))
  assert.match(names.json, /mts-readiness-archive-2026-07-20T12-34-56\.json/)
  assert.match(names.md, /\.md$/)
})
