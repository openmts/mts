import assert from 'node:assert/strict'
import test from 'node:test'
import {
  acceptancePackFilenames,
  buildAcceptancePack,
  formatAcceptancePackMarkdown,
  ACCEPTANCE_PACK_KIND,
} from './acceptancePack.ts'
import { buildReadinessArchive } from './readinessArchive.ts'

function sampleArchive() {
  return buildReadinessArchive({
    operator: 'alice',
    note: 'drill',
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
}

test('buildAcceptancePack aggregates archive client server ops', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive(),
    client: {
      name: 'mts-dashboard',
      version: '1.2.3',
      mode: 'test',
      base: '/',
      apiBase: '',
    },
    server: { version: '0.9.0', commit: 'abc', built_at: '2026-07-01' },
    opsActions: [
      { id: '1', kind: 'flush', status: 'ok', message: 'flushed', at: Date.parse('2026-07-20T10:00:00.000Z') },
    ],
    operator: 'bob',
    note: 'handoff',
    now: '2026-07-20T13:00:00.000Z',
  })
  assert.equal(pack.kind, ACCEPTANCE_PACK_KIND)
  assert.equal(pack.version, 1)
  assert.equal(pack.operator, 'bob')
  assert.equal(pack.client.version, '1.2.3')
  assert.equal(pack.server?.version, '0.9.0')
  assert.equal(pack.readiness.score.total, 82)
  assert.equal(pack.ops_actions.length, 1)
  assert.match(pack.disclaimer, /不代表生产人工验收/)
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /验收材料包/)
  assert.match(md, /1\.2\.3/)
  assert.match(md, /0\.9\.0/)
  assert.match(md, /flush/)
  assert.match(md, /82%/)
})

test('buildAcceptancePack allows missing server', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive(),
    client: { name: 'mts-dashboard', version: '0.0.0', mode: 'dev', base: '/', apiBase: '' },
    now: '2026-07-20T13:00:00.000Z',
  })
  assert.equal(pack.server, null)
  assert.equal(pack.ops_actions.length, 0)
  assert.match(formatAcceptancePackMarkdown(pack), /服务端：未加载/)
})

test('acceptancePackFilenames stamp', () => {
  const names = acceptancePackFilenames(new Date('2026-07-20T12:34:56.000Z'))
  assert.match(names.json, /mts-acceptance-pack-2026-07-20T12-34-56\.json/)
  assert.match(names.md, /\.md$/)
})
