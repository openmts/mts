import assert from 'node:assert/strict'
import test from 'node:test'
import {
  acceptancePackFilenames,
  buildAcceptancePack,
  formatAcceptancePackMarkdown,
  ACCEPTANCE_PACK_KIND,
} from './acceptancePack.ts'
import { buildReadinessArchive } from './readinessArchive.ts'

function sampleArchive(locale: 'zh' | 'en' = 'zh') {
  return buildReadinessArchive({
    locale,
    operator: 'alice',
    note: 'drill',
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
    downsample_status_summary: { total: 1, error: 1, lagging: 0, max_lag_seconds: 0 },
  })
}

test('buildAcceptancePack aggregates archive client server ops', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive('zh'),
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
  assert.equal(pack.version, 2)
  assert.equal(pack.locale, 'zh')
  assert.equal(pack.operator, 'bob')
  assert.equal(pack.client.version, '1.2.3')
  assert.equal(pack.server?.version, '0.9.0')
  assert.equal(pack.readiness.score.total, 82)
  assert.equal(pack.ops_actions.length, 1)
  assert.match(pack.disclaimer, /不代表生产人工验收/)
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /Downsample status summary/)
  assert.match(md, /Commercial handoff/)
  assert.match(md, /验收材料包/)
  assert.match(md, /1\.2\.3/)
  assert.match(md, /0\.9\.0/)
  assert.match(md, /flush/)
  assert.match(md, /82%/)
  assert.match(md, /边缘 HTTPS \/ TLS/)
  assert.ok(pack.deploy_kit.count >= 5)
  assert.equal(pack.deploy_kit.manual_signoff_required, true)
  assert.ok(pack.deploy_kit.items.some((x) => x.id === 'nginx-https'))
  assert.match(md, /部署材料包索引|nginx-https/)
  assert.equal(pack.signoff_completeness.complete, false)
  assert.equal(pack.signoff_completeness.filled_count, 0)
  assert.match(md, /签核备注完整性/)
  assert.match(md, /不计入就绪评分/)
  assert.ok(pack.export_preflight.warn_count >= 1)
  assert.ok(pack.export_preflight.items.some((i) => i.id === 'signoff'))
  assert.match(md, /导出前预检/)
  assert.match(md, /预检不阻止导出/)
  assert.ok(pack.data_contract)
  assert.equal(typeof pack.data_contract.summary_line, 'string')
  assert.equal(typeof pack.data_contract.loaded, 'boolean')
  assert.match(md, /数据面契约/)
  assert.match(md, /data_contract|max_write|unavailable|complete|features=/)
})

test('acceptance pack data_contract reflects commercial handoff features', () => {
  const archive = sampleArchive('zh')
  archive.commercial_handoff = {
    password_policy: archive.commercial_handoff!.password_policy,
    session_calibration: archive.commercial_handoff!.session_calibration,
    data_contract: {
      loaded: true,
      path: '/api/v1/data/contract',
      version: 1,
      maxWritePoints: 5000,
      defaultQueryLimit: 1000,
      maxQueryLimit: 10000,
      features: [
        { id: 'write_accepted_points', path: '/api/v1/data/write', enabled: true, description: '' },
        { id: 'write_response_mode', path: '/api/v1/data/write', enabled: true, description: '' },
        { id: 'write_response_retention', path: '/api/v1/data/write', enabled: true, description: '' },
        { id: 'query_result_meta', path: '/api/v1/data/query/rows', enabled: true, description: '' },
        { id: 'query_stats_path', path: '/api/v1/data/query/stats', enabled: true, description: '' },
        { id: 'query_stream_end_meta', path: '/api/v1/data/query/stream', enabled: true, description: '' },
        { id: 'delete_response_meta', path: '/api/v1/data/delete', enabled: true, description: '' },
        { id: 'data_limits', path: '/api/v1/data/limits', enabled: true, description: '' },
        { id: 'meta_list_path', path: '/api/v1/data/databases', enabled: true, description: '' },
      ],
      enabledCount: 7,
      totalFeatures: 7,
      missingRequired: [],
      complete: true,
    },
  }
  const pack = buildAcceptancePack({
    archive,
    client: { name: 'mts-dashboard', version: '1.0.0', mode: 'test', base: '/', apiBase: '' },
    locale: 'zh',
  })
  assert.equal(pack.data_contract.loaded, true)
  assert.equal(pack.data_contract.complete, true)
  assert.equal(pack.data_contract.max_write_points, 5000)
  assert.match(pack.data_contract.summary_line, /complete/)
  assert.ok(pack.export_preflight.items.some((i) => i.id === 'data-contract' && i.level === 'ok'))
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /数据面契约/)
  assert.match(md, /能力齐全|complete/)
})


test('acceptance pack signoff completeness reflects archive notes', () => {
  const archive = sampleArchive('zh')
  archive.state.signoffNotes = {
    edgeHttps: 'cert ok',
    backupOffsite: 'rsync ok',
    backupAlert: 'webhook',
  }
  archive.signoff_notes = {
    edgeHttps: 'cert ok',
    backupOffsite: 'rsync ok',
    backupAlert: 'webhook',
  }
  const pack = buildAcceptancePack({
    archive,
    client: { name: 'mts-dashboard', version: '1.0.0', mode: 'test', base: '/', apiBase: '' },
    locale: 'zh',
  })
  assert.equal(pack.signoff_completeness.complete, true)
  assert.equal(pack.signoff_completeness.filled_count, 3)
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /已齐/)
  assert.match(md, /边缘证书/)
})

test('buildAcceptancePack allows missing server', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive('zh'),
    client: { name: 'mts-dashboard', version: '0.0.0', mode: 'dev', base: '/', apiBase: '' },
    now: '2026-07-20T13:00:00.000Z',
  })
  assert.equal(pack.server, null)
  assert.equal(pack.ops_actions.length, 0)
  assert.match(formatAcceptancePackMarkdown(pack), /服务端：未加载/)
})

test('acceptance pack english includes localized catalog title', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive('en'),
    client: { name: 'mts-dashboard', version: '1.0.0', mode: 'prod', base: '/', apiBase: '' },
    locale: 'en',
  })
  assert.equal(pack.locale, 'en')
  assert.match(pack.disclaimer, /does not prove production human acceptance/i)
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /commercial acceptance pack/i)
  assert.match(md, /Edge HTTPS \/ TLS/)
  assert.match(md, /catalog\.sample: https-edge — Edge HTTPS \/ TLS/)
  assert.ok(pack.deploy_kit.items.some((x) => x.id === 'cron-backup' && /cron/i.test(x.title)))
  assert.match(md, /Deployment kit index/i)
  assert.match(md, /nginx-https/)
})

test('acceptancePackFilenames stamp', () => {
  const names = acceptancePackFilenames(new Date('2026-07-20T12:34:56.000Z'))
  assert.match(names.json, /mts-acceptance-pack-2026-07-20T12-34-56\.json/)
  assert.match(names.md, /\.md$/)
})

test('acceptance pack includes commercial handoff', () => {
  const pack = buildAcceptancePack({
    archive: sampleArchive('zh'),
    client: { name: 'mts-dashboard', version: '1.0.0', mode: 'test', base: '/', apiBase: '' },
    locale: 'zh',
  })
  assert.ok(pack.readiness.commercial_handoff?.password_policy.min_length)
  const md = formatAcceptancePackMarkdown(pack)
  assert.match(md, /Commercial handoff/)
  assert.match(md, /password_policy/)
})

