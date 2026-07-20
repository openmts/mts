import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEPLOY_TEMPLATES,
  deployKitFilename,
  deployTemplateById,
  formatDeployKitMarkdown,
  buildDeployKitSummary,
} from './deployTemplates.ts'

test('deploy templates cover cert and scheduler surfaces', () => {
  const ids = DEPLOY_TEMPLATES.map((t) => t.id)
  for (const need of [
    'nginx-https',
    'cert-acceptance-checks',
    'cron-backup',
    'systemd-backup-service',
    'systemd-backup-timer',
    'backup-env',
  ]) {
    assert.ok(ids.includes(need), need)
  }
  assert.ok(deployTemplateById('nginx-https')?.body.includes('Strict-Transport-Security'))
  assert.ok(deployTemplateById('cron-backup')?.body.includes('mts-backup.sh'))
})

test('formatDeployKitMarkdown is bilingual and includes sample bodies', () => {
  const zh = formatDeployKitMarkdown('zh')
  const en = formatDeployKitMarkdown('en')
  assert.match(zh, /部署材料包/)
  assert.match(en, /deployment kit/i)
  assert.match(en, /Nginx HTTPS \+ HSTS sample/)
  assert.match(en, /does \*\*not\*\* mark edge HTTPS/)
  assert.match(zh, /mts-backup.service/)
  assert.match(zh, /联调清单|附录/)
  assert.match(en, /runbook drill|Appendix/i)
  assert.match(deployKitFilename(new Date('2026-07-20T12:34:56.000Z')), /mts-deploy-kit-2026-07-20T12-34-56\.md/)
})


test('buildDeployKitSummary localizes titles and marks manual signoff', () => {
  const zh = buildDeployKitSummary('zh')
  const en = buildDeployKitSummary('en')
  assert.equal(zh.manual_signoff_required, true)
  assert.ok(zh.count >= 5)
  assert.ok(zh.items.some((x) => x.id === 'nginx-https' && /Nginx/.test(x.title)))
  assert.equal(en.items.find((x) => x.id === 'nginx-https')?.title, 'Nginx HTTPS + HSTS sample')
  assert.match(en.note, /does not complete/i)
})

test('deploy kit includes offsite rsync and alert hook samples', () => {
  const zh = buildDeployKitSummary('zh')
  assert.ok(zh.count >= 7)
  assert.ok(zh.items.some((x) => x.id === 'rsync-offsite'))
  assert.ok(zh.items.some((x) => x.id === 'backup-alert-hook'))
  const md = formatDeployKitMarkdown('zh')
  assert.match(md, /rsync/)
  assert.match(md, /mts-backup-alert/)
  assert.match(md, /不代表/)
})

test('formatDeployKitMarkdown can omit drill appendix', () => {
  const zh = formatDeployKitMarkdown('zh', undefined, { includeDrill: false })
  assert.doesNotMatch(zh, /附录：Runbook 联调清单/)
})
