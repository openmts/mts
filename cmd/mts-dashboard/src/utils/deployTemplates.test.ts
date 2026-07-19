import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEPLOY_TEMPLATES,
  deployKitFilename,
  deployTemplateById,
  formatDeployKitMarkdown,
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
  assert.match(deployKitFilename(new Date('2026-07-20T12:34:56.000Z')), /mts-deploy-kit-2026-07-20T12-34-56\.md/)
})
