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
      deployKit: {},
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
    downsample_status_summary: { total: 3, error: 1, lagging: 2, max_lag_seconds: 42 },
  })
  assert.equal(a.kind, 'mts.readiness.archive')
  assert.equal(a.downsample_status_summary?.error, 1)
  assert.equal(a.downsample_status_summary?.max_lag_seconds, 42)
  assert.equal(a.locale, 'zh')
  assert.equal(a.operator, 'alice')
  assert.equal(a.score.total, 82)
  assert.deepEqual(a.checklist.production, ['https-edge'])
  assert.deepEqual(a.checklist.edgeHttps, ['tls-terminate'])
  assert.ok(a.catalog.production.some((x) => x.id === 'https-edge' && x.done && /边缘|HTTPS/.test(x.title)))
  assert.ok(a.deploy_kit.count >= 5)
  assert.equal(a.deploy_kit.manual_signoff_required, true)
  assert.ok(a.deploy_kit.items.some((x) => x.id === 'nginx-https'))
  assert.deepEqual(a.deploy_kit_local_hints, [])
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /alice/)
  assert.match(md, /82%/)
  assert.match(md, /http_tls/)
  assert.match(md, /weekly drill/)
  assert.match(md, /边缘 HTTPS \/ TLS|Edge HTTPS/)
  assert.match(md, /Downsample status summary/)
  assert.match(md, /max_lag_seconds: 42/)
  assert.ok(a.commercial_handoff?.password_policy.min_length)
  assert.match(md, /Commercial handoff/)
  assert.match(md, /password_policy/)
})

test('buildReadinessArchive commercial handoff session merge', () => {
  const now = Date.parse('2026-07-22T12:00:00.000Z')
  const a = buildReadinessArchive({
    operator: 'ops',
    now: new Date(now).toISOString(),
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: {},
      updatedAt: new Date(now).toISOString(),
    },
    score: { checklist: 0, edgeHttps: 0, backupSchedule: 0, doctor: 0, total: 0, reasons: [] },
    doctor: { loaded: false },
    session_expires_at: new Date(now + 600_000).toISOString(),
    session_remaining_seconds: 90,
    session_checked_at_ms: now,
    session_sample_source: 'session' as const,
      session_server_time_unix: Math.floor(now / 1000) - 2,
  })
  assert.equal(a.commercial_handoff?.session_calibration.calibration_source, 'session')
  assert.equal(a.commercial_handoff?.session_calibration.clock_skew_seconds, 2)
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /session_calibration/)
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
      deployKit: {},
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
  assert.match(md, /Deployment kit index/i)
  assert.match(md, /nginx-https/)
})

test('archive includes deploy kit local hints without scoring claim', () => {
  const a = buildReadinessArchive({
    locale: 'zh',
    operator: 'ops',
    now: '2026-07-20T12:00:00.000Z',
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: { reviewed: true, downloaded: true },
      updatedAt: '2026-07-20T11:00:00.000Z',
    },
    score: {
      checklist: 0,
      edgeHttps: 0,
      backupSchedule: 0,
      doctor: 40,
      total: 10,
      reasons: ['doctor_unavailable'],
    },
    doctor: { loaded: false },
  })
  assert.deepEqual(a.deploy_kit_local_hints, ['downloaded', 'reviewed'])
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /本地提醒勾选/)
  assert.match(md, /downloaded, reviewed|reviewed, downloaded|downloaded/)
  assert.match(md, /人工签核仍为部署侧必做/)
})

test('archiveFilenames uses iso-like stamp', () => {
  const names = archiveFilenames(new Date('2026-07-20T12:34:56.000Z'))
  assert.match(names.json, /mts-readiness-archive-2026-07-20T12-34-56\.json/)
  assert.match(names.md, /\.md$/)
})

test('archive includes signoff notes without claiming acceptance', () => {
  const a = buildReadinessArchive({
    locale: 'zh',
    operator: 'ops',
    now: '2026-07-20T13:00:00.000Z',
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: {},
      signoffNotes: {
        edgeHttps: 'openssl leaf expires 2027-01-01',
        backupOffsite: 'rsync to backup@host keep=7',
        backupAlert: 'webhook #ops-alerts',
      },
    },
    score: {
      checklist: 0,
      edgeHttps: 0,
      backupSchedule: 0,
      doctor: 40,
      total: 10,
      reasons: ['doctor_unavailable'],
    },
    doctor: { loaded: false },
  })
  assert.equal(a.signoff_notes.edgeHttps, 'openssl leaf expires 2027-01-01')
  assert.equal(a.signoff_notes.backupOffsite, 'rsync to backup@host keep=7')
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /部署侧签核证据备注/)
  assert.match(md, /openssl leaf expires/)
  assert.match(md, /webhook #ops-alerts/)
  assert.match(md, /不代表生产人工验收已完成/)
})

test('archive includes export preflight summary', () => {
  const a = buildReadinessArchive({
    locale: 'zh',
    operator: 'ops',
    now: '2026-07-20T14:00:00.000Z',
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: { reviewed: true },
      signoffNotes: { edgeHttps: 'only edge' },
    },
    score: {
      checklist: 50,
      edgeHttps: 100,
      backupSchedule: 0,
      doctor: 40,
      total: 48,
      reasons: ['checklist_incomplete', 'backup_schedule_incomplete', 'doctor_unavailable'],
    },
    doctor: { loaded: false },
  })
  assert.ok(a.export_preflight.warn_count >= 1)
  assert.ok(a.export_preflight.items.some((i) => i.id === 'signoff'))
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /导出前预检/)
  assert.match(md, /预检不阻止导出/)
})

test('buildReadinessArchive includes api_paths and doctor path', () => {
  const a = buildReadinessArchive({
    operator: 'ops',
    note: 'n',
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: {},
      signoffNotes: { edgeHttps: '', backupOffsite: '', backupAlert: '' },
      updatedAt: '2026-07-23T00:00:00.000Z',
    },
    score: { total: 10, checklist: 0, edgeHttps: 0, backupSchedule: 0, doctor: 10, reasons: [] },
    doctor: {
      loaded: true,
      ok: true,
      path: '/api/v1/admin/doctor',
      http_tls_enabled: true,
      warn_count: 0,
      checks: [],
    },
    api_paths: {
      doctor: '/api/v1/admin/doctor',
      version: '/api/v1/admin/version',
    },
  })
  assert.equal(a.doctor.path, '/api/v1/admin/doctor')
  assert.equal(a.api_paths?.doctor, '/api/v1/admin/doctor')
  assert.equal(a.api_paths?.version, '/api/v1/admin/version')
  assert.ok(a.api_paths?.ops_status?.includes('ops-status'))
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /API paths/)
  assert.match(md, /doctor: \/api\/v1\/admin\/doctor/)
})

test('buildReadinessArchive includes storage_drill events', () => {
  const a = buildReadinessArchive({
    operator: 'op',
    note: '',
    state: {
      production: {},
      edgeHttps: {},
      backupSchedule: {},
      deployKit: {},
      updatedAt: '2026-07-23T00:00:00.000Z',
    },
    score: { checklist: 0, edgeHttps: 0, backupSchedule: 0, doctor: 0, total: 0, reasons: [] },
    doctor: { loaded: false },
    storage_drill: {
      version: 1,
      updated_at: '2026-07-23T00:00:00Z',
      events: [{
        kind: 'validate',
        at: '2026-07-23T00:00:00Z',
        path: '/api/v1/admin/storage/validate',
        ok: true,
        summary: 'healthy',
      }],
    },
  })
  assert.equal(a.storage_drill?.events?.[0]?.kind, 'validate')
  const md = formatReadinessArchiveMarkdown(a)
  assert.match(md, /Storage drill/)
  assert.match(md, /validate/)
})
