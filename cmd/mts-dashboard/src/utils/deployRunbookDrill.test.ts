import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEPLOY_DRILL_STEPS,
  appendDrillToDeployKitMarkdown,
  buildDeployRunbookDrillSummary,
  deployDrillStepsByArea,
  deployRunbookDrillFilename,
  formatDeployDrillAreaLabel,
  formatDeployRunbookDrillMarkdown,
} from './deployRunbookDrill.ts'

test('drill steps cover three deployment-side areas', () => {
  const areas = new Set(DEPLOY_DRILL_STEPS.map((s) => s.area))
  assert.ok(areas.has('edge_https'))
  assert.ok(areas.has('scheduler'))
  assert.ok(areas.has('offsite_backup'))
  assert.ok(deployDrillStepsByArea('edge_https').length >= 2)
  assert.ok(deployDrillStepsByArea('scheduler').length >= 2)
  assert.ok(deployDrillStepsByArea('offsite_backup').length >= 2)
})

test('summary is not scored and localizes', () => {
  const zh = buildDeployRunbookDrillSummary('zh')
  const en = buildDeployRunbookDrillSummary('en')
  assert.equal(zh.scored, false)
  assert.equal(zh.manual_signoff_required, true)
  assert.equal(zh.count, DEPLOY_DRILL_STEPS.length)
  assert.ok(zh.items.some((i) => /证书|HTTPS/.test(i.title)))
  assert.ok(en.items.some((i) => /certificate|HTTPS|HSTS/i.test(i.title)))
  assert.match(en.note, /does not complete/i)
})

test('markdown includes runbooks evidence and signoff', () => {
  const zh = formatDeployRunbookDrillMarkdown('zh', DEPLOY_DRILL_STEPS, new Date('2026-07-20T10:00:00.000Z'))
  const en = formatDeployRunbookDrillMarkdown('en', DEPLOY_DRILL_STEPS, new Date('2026-07-20T10:00:00.000Z'))
  assert.match(zh, /联调清单/)
  assert.match(zh, /dashboard-production-runbook\.md/)
  assert.match(zh, /backup-orchestration\.md/)
  assert.match(zh, /签核/)
  assert.match(zh, /不计入/)
  assert.match(en, /runbook drill/i)
  assert.match(en, /Sign-off/)
  assert.match(en, /does \*\*not\*\* count/i)
  assert.equal(formatDeployDrillAreaLabel('scheduler', 'en'), 'cron / systemd schedule')
  assert.match(deployRunbookDrillFilename(new Date('2026-07-20T12:34:56.000Z')), /mts-deploy-runbook-drill-2026-07-20T12-34-56\.md/)
})

test('appendDrillToDeployKitMarkdown appends appendix', () => {
  const kit = '# kit\n\nbody\n'
  const out = appendDrillToDeployKitMarkdown(kit, 'zh')
  assert.match(out, /^# kit/)
  assert.match(out, /附录：Runbook 联调清单/)
  assert.match(out, /边缘 HTTPS/)
})
