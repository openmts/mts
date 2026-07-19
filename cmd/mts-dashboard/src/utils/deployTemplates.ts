/** 部署侧证书验收 / cron·systemd 样例（纯数据；人工执行，不伪造成自动化完成） */

import type { LocaleCode, LocalizedText } from './localizedText.ts'
import { textForLocale } from './localizedText.ts'

export interface DeployTemplate {
  id: string
  title: LocalizedText
  description: LocalizedText
  filename: string
  language: 'nginx' | 'ini' | 'shell' | 'cron' | 'env' | 'markdown'
  /** 部署侧人工步骤，Dashboard 仅提供样例 */
  body: string
}

export const DEPLOY_TEMPLATES: DeployTemplate[] = [
  {
    id: 'nginx-https',
    title: { zh: 'Nginx HTTPS + HSTS 样例', en: 'Nginx HTTPS + HSTS sample' },
    description: {
      zh: '边缘 TLS 终止与 HSTS 响应头样例；证书路径与上游地址请按环境替换。',
      en: 'Sample edge TLS termination and HSTS header; replace cert paths and upstream for your environment.',
    },
    filename: 'mts-nginx-https.conf.sample',
    language: 'nginx',
    body: `server {
  listen 80;
  server_name mts.example.com;
  return 308 https://$host$request_uri;
}

server {
  listen 443 ssl http2;
  server_name mts.example.com;

  ssl_certificate     /etc/ssl/mts.crt;
  ssl_certificate_key /etc/ssl/mts.key;
  add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Request-ID $request_id;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
`,
  },
  {
    id: 'cert-acceptance-checks',
    title: { zh: '证书/HSTS 人工验收命令', en: 'Certificate / HSTS manual check commands' },
    description: {
      zh: '浏览器外的命令行核验样例；通过不代表生产人工验收已签字完成。',
      en: 'CLI verification samples outside the browser; success does not mean production acceptance is signed off.',
    },
    filename: 'mts-cert-acceptance-checks.sh.sample',
    language: 'shell',
    body: `#!/usr/bin/env bash
set -euo pipefail
HOST="\${MTS_PUBLIC_HOST:-mts.example.com}"

echo "== TLS certificate =="
echo | openssl s_client -servername "$HOST" -connect "$HOST:443" 2>/dev/null | openssl x509 -noout -subject -issuer -dates

echo "== HTTPS headers (expect HSTS after full-site HTTPS) =="
curl -fsSI "https://$HOST/" | tr -d '\\r' | grep -Ei 'HTTP/|strict-transport-security|content-security-policy' || true

echo "== HTTP redirect =="
curl -fsSI "http://$HOST/" | tr -d '\\r' | head -n 5

echo "== Doctor (admin token required) =="
curl -fsS -H "Authorization: Bearer \${MTS_ADMIN_TOKEN:?set MTS_ADMIN_TOKEN}" \\
  "https://$HOST/api/v1/admin/doctor" | head -c 2000
echo
`,
  },
  {
    id: 'backup-env',
    title: { zh: '备份脚本环境变量样例', en: 'Backup script environment sample' },
    description: {
      zh: '供 systemd EnvironmentFile 或 cron 前缀使用；密钥勿提交仓库。',
      en: 'For systemd EnvironmentFile or cron prefixes; do not commit secrets.',
    },
    filename: 'mts-backup.env.sample',
    language: 'env',
    body: `MTS_BASE_URL=https://mts.example.com
MTS_ADMIN_TOKEN=replace-me
MTS_BACKUP_REMOTE=backup@host:/var/backups/mts
# optional:
# MTS_BACKUP_KEEP=7
# MTS_BACKUP_SKIP_REMOTE=0
`,
  },
  {
    id: 'cron-backup',
    title: { zh: 'cron 定时备份样例', en: 'cron scheduled backup sample' },
    description: {
      zh: '主机 crontab 条目；日志与告警通道由部署侧接入。',
      en: 'Host crontab entry; wire logs and alerts on the deployment side.',
    },
    filename: 'mts-backup.cron.sample',
    language: 'cron',
    body: `15 * * * * /usr/bin/env bash -lc 'set -a; source /etc/mts/backup.env; set +a; /opt/mts/scripts/mts-backup.sh' >>/var/log/mts-backup.log 2>&1
`,
  },
  {
    id: 'systemd-backup-service',
    title: { zh: 'systemd oneshot 服务样例', en: 'systemd oneshot service sample' },
    description: {
      zh: '写入 /etc/systemd/system/mts-backup.service 后 systemctl daemon-reload。',
      en: 'Install to /etc/systemd/system/mts-backup.service then systemctl daemon-reload.',
    },
    filename: 'mts-backup.service.sample',
    language: 'ini',
    body: `[Unit]
Description=MTS data_dir backup orchestration
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
EnvironmentFile=-/etc/mts/backup.env
ExecStart=/opt/mts/scripts/mts-backup.sh
Nice=10
# Optional failure alerting hook:
# ExecStopPost=/usr/local/bin/mts-backup-alert.sh
`,
  },
  {
    id: 'systemd-backup-timer',
    title: { zh: 'systemd timer 样例', en: 'systemd timer sample' },
    description: {
      zh: '写入 /etc/systemd/system/mts-backup.timer 后 enable --now。',
      en: 'Install to /etc/systemd/system/mts-backup.timer then enable --now.',
    },
    filename: 'mts-backup.timer.sample',
    language: 'ini',
    body: `[Unit]
Description=Hourly MTS backup

[Timer]
OnCalendar=hourly
Persistent=true
RandomizedDelaySec=3m

[Install]
WantedBy=timers.target
`,
  },
]

export function deployTemplateById(id: string, templates = DEPLOY_TEMPLATES): DeployTemplate | undefined {
  return templates.find((t) => t.id === id)
}

export function formatDeployTemplateLabel(tpl: DeployTemplate, locale: LocaleCode = 'zh'): string {
  return textForLocale(tpl.title, locale)
}

export function formatDeployKitMarkdown(locale: LocaleCode = 'zh', templates = DEPLOY_TEMPLATES): string {
  const head =
    locale === 'en'
      ? [
          '# MTS deployment kit (samples)',
          '',
          'These snippets are for host-side install and manual acceptance.',
          'Copying or downloading them does **not** mark edge HTTPS or off-host backup as complete.',
          '',
          'Related docs: `docs/ops/dashboard-production-runbook.md`, `docs/ops/backup-orchestration.md`.',
          '',
        ]
      : [
          '# MTS 部署材料包（样例）',
          '',
          '以下片段用于主机侧安装与人工验收。',
          '复制或下载**不代表**边缘 HTTPS 或异地备份已完成。',
          '',
          '相关文档：`docs/ops/dashboard-production-runbook.md`、`docs/ops/backup-orchestration.md`。',
          '',
        ]
  const lines = [...head]
  for (const tpl of templates) {
    lines.push(`## ${textForLocale(tpl.title, locale)}`)
    lines.push('')
    lines.push(textForLocale(tpl.description, locale))
    lines.push('')
    lines.push(`\`${tpl.filename}\``)
    lines.push('')
    lines.push('```' + (tpl.language === 'markdown' ? '' : tpl.language))
    lines.push(tpl.body.replace(/\n$/, ''))
    lines.push('```')
    lines.push('')
  }
  lines.push('---')
  lines.push('')
  lines.push(
    locale === 'en'
      ? '_Generated by Dashboard readiness center; deployment-side human sign-off remains required._'
      : '_由 Dashboard 就绪中心生成；部署侧人工签核仍为必做项。_',
  )
  lines.push('')
  return lines.join('\n')
}

export function deployKitFilename(at = new Date()): string {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return `mts-deploy-kit-${stamp}.md`
}
