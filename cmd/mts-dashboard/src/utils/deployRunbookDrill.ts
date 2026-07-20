/** 部署侧 runbook 联调清单（纯数据/纯函数；不计分，不伪造成验收完成） */

import type { LocaleCode, LocalizedText } from './localizedText.ts'
import { textForLocale } from './localizedText.ts'

export type DeployDrillArea = 'edge_https' | 'scheduler' | 'offsite_backup'

export interface DeployDrillStep {
  id: string
  area: DeployDrillArea
  title: LocalizedText
  /** 在目标环境执行的动作说明 */
  action: LocalizedText
  /** 建议证据 / 留档 */
  evidence: LocalizedText
  /** 关联仓库文档路径（相对 repo root） */
  runbookPaths: string[]
  /** 可选：对应部署材料样例 id */
  templateIds?: string[]
}

export interface DeployDrillSummary {
  version: 1
  kind: 'mts.deploy.runbook.drill'
  manual_signoff_required: true
  scored: false
  count: number
  areas: DeployDrillArea[]
  items: Array<{
    id: string
    area: DeployDrillArea
    title: string
    action: string
    evidence: string
    runbook_paths: string[]
    template_ids: string[]
  }>
  note: string
}

export const DEPLOY_RUNBOOK_PATHS = {
  production: 'docs/ops/dashboard-production-runbook.md',
  backup: 'docs/ops/backup-orchestration.md',
} as const

export const DEPLOY_DRILL_STEPS: DeployDrillStep[] = [
  {
    id: 'edge-cert-present',
    area: 'edge_https',
    title: {
      zh: '边缘证书部署与链完整',
      en: 'Edge certificate installed with full chain',
    },
    action: {
      zh: '在反向代理/LB 配置有效证书与私钥；用 openssl/浏览器确认链与到期日。',
      en: 'Install valid cert/key on reverse proxy/LB; verify chain and expiry via openssl/browser.',
    },
    evidence: {
      zh: '验收人/日期、证书 subject/issuer/notAfter、变更单号。',
      en: 'Owner/date, cert subject/issuer/notAfter, change ticket.',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.production],
    templateIds: ['nginx-https', 'cert-acceptance-checks'],
  },
  {
    id: 'edge-http-redirect-hsts',
    area: 'edge_https',
    title: {
      zh: 'HTTP→HTTPS 跳转与 HSTS',
      en: 'HTTP→HTTPS redirect and HSTS',
    },
    action: {
      zh: '确认明文跳转 301/308；全站 HTTPS 后启用 HSTS；复测 curl -I 与 doctor TLS 提示。',
      en: 'Confirm cleartext 301/308; enable HSTS only after full-site HTTPS; recheck curl -I and doctor TLS hints.',
    },
    evidence: {
      zh: 'curl 响应头摘要（含 Strict-Transport-Security）、截图或日志。',
      en: 'curl header excerpt (incl. Strict-Transport-Security), screenshot or log.',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.production],
    templateIds: ['nginx-https', 'cert-acceptance-checks'],
  },
  {
    id: 'scheduler-systemd-or-cron',
    area: 'scheduler',
    title: {
      zh: 'cron 或 systemd 定时备份实装',
      en: 'Install cron or systemd backup schedule',
    },
    action: {
      zh: '在目标主机写入 EnvironmentFile + service/timer 或 crontab；手动触发一次备份脚本。',
      en: 'Install EnvironmentFile + service/timer or crontab on target host; trigger backup script once.',
    },
    evidence: {
      zh: 'unit/crontab 路径、最近一次成功时间、journalctl/cron 日志片段。',
      en: 'unit/crontab path, last success time, journalctl/cron log snippet.',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.backup, DEPLOY_RUNBOOK_PATHS.production],
    templateIds: ['backup-env', 'cron-backup', 'systemd-backup-service', 'systemd-backup-timer'],
  },
  {
    id: 'restore-drill',
    area: 'scheduler',
    title: {
      zh: '恢复演练（旁路/非生产）',
      en: 'Restore drill (side-path / non-prod)',
    },
    action: {
      zh: '用快照做旁路恢复或 storage restore-drill；记录 RTO/RPO 与失败回退。',
      en: 'Side-path restore or storage restore-drill; record RTO/RPO and rollback path.',
    },
    evidence: {
      zh: '演练报告/归档 Markdown、耗时、结果（通过/问题列表）。',
      en: 'Drill report/archive markdown, duration, result (pass/issues).',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.backup, DEPLOY_RUNBOOK_PATHS.production],
    templateIds: [],
  },
  {
    id: 'offsite-copy',
    area: 'offsite_backup',
    title: {
      zh: '跨主机 / 异地拷贝',
      en: 'Off-host / offsite copy',
    },
    action: {
      zh: '将备份目录 rsync/对象存储到独立故障域；验证远端可读与保留策略。',
      en: 'rsync/object-copy backups to a separate failure domain; verify remote readability and retention.',
    },
    evidence: {
      zh: '远端主机/路径、最近成功时间、保留份数、传输日志。',
      en: 'Remote host/path, last success time, retention count, transfer log.',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.backup],
    templateIds: ['rsync-offsite'],
  },
  {
    id: 'backup-failure-alert',
    area: 'offsite_backup',
    title: {
      zh: '备份失败告警通道',
      en: 'Backup failure alert channel',
    },
    action: {
      zh: '接入邮件/IM/webhook；故意失败或演练触发一次，确认值班可见。',
      en: 'Wire email/IM/webhook; force-fail or drill once and confirm on-call visibility.',
    },
    evidence: {
      zh: '告警通道配置名、演练触发记录、值班回执。',
      en: 'Alert channel name, drill trigger record, on-call ack.',
    },
    runbookPaths: [DEPLOY_RUNBOOK_PATHS.backup, DEPLOY_RUNBOOK_PATHS.production],
    templateIds: ['backup-alert-hook'],
  },
]

export function deployDrillStepsByArea(
  area: DeployDrillArea,
  steps = DEPLOY_DRILL_STEPS,
): DeployDrillStep[] {
  return steps.filter((s) => s.area === area)
}

export function formatDeployDrillAreaLabel(area: DeployDrillArea, locale: LocaleCode = 'zh'): string {
  if (locale === 'en') {
    switch (area) {
      case 'edge_https':
        return 'Edge HTTPS / HSTS'
      case 'scheduler':
        return 'cron / systemd schedule'
      case 'offsite_backup':
        return 'Offsite backup + alerts'
      default:
        return area
    }
  }
  switch (area) {
    case 'edge_https':
      return '边缘 HTTPS / HSTS'
    case 'scheduler':
      return 'cron / systemd 编排'
    case 'offsite_backup':
      return '异地备份 + 告警'
    default:
      return area
  }
}

export function buildDeployRunbookDrillSummary(
  locale: LocaleCode = 'zh',
  steps = DEPLOY_DRILL_STEPS,
): DeployDrillSummary {
  const note =
    locale === 'en'
      ? 'Local checklist / copy / download does not complete deployment-side acceptance.'
      : '本地勾选、复制或下载不代表部署侧验收完成。'
  return {
    version: 1,
    kind: 'mts.deploy.runbook.drill',
    manual_signoff_required: true,
    scored: false,
    count: steps.length,
    areas: ['edge_https', 'scheduler', 'offsite_backup'],
    items: steps.map((s) => ({
      id: s.id,
      area: s.area,
      title: textForLocale(s.title, locale),
      action: textForLocale(s.action, locale),
      evidence: textForLocale(s.evidence, locale),
      runbook_paths: [...s.runbookPaths],
      template_ids: [...(s.templateIds ?? [])],
    })),
    note,
  }
}

export function formatDeployRunbookDrillMarkdown(
  locale: LocaleCode = 'zh',
  steps = DEPLOY_DRILL_STEPS,
  at = new Date(),
): string {
  const iso = at.toISOString()
  const head =
    locale === 'en'
      ? [
          '# MTS deployment runbook drill checklist',
          '',
          `Generated at: ${iso}`,
          '',
          'Use this on the **target environment**. Completing steps in Dashboard does **not** count toward readiness score and does **not** mean production acceptance is signed off.',
          '',
          'Related runbooks:',
          `- \`${DEPLOY_RUNBOOK_PATHS.production}\``,
          `- \`${DEPLOY_RUNBOOK_PATHS.backup}\``,
          '',
        ]
      : [
          '# MTS 部署 Runbook 联调清单',
          '',
          `生成时间：${iso}`,
          '',
          '请在**目标环境**执行。Dashboard 内勾选/复制/下载**不计入**就绪评分，也**不代表**生产验收已签字完成。',
          '',
          '相关 runbook：',
          `- \`${DEPLOY_RUNBOOK_PATHS.production}\``,
          `- \`${DEPLOY_RUNBOOK_PATHS.backup}\``,
          '',
        ]

  const lines = [...head]
  const areas: DeployDrillArea[] = ['edge_https', 'scheduler', 'offsite_backup']
  for (const area of areas) {
    lines.push(`## ${formatDeployDrillAreaLabel(area, locale)}`)
    lines.push('')
    const group = deployDrillStepsByArea(area, steps)
    let n = 1
    for (const s of group) {
      lines.push(`### ${n}. ${textForLocale(s.title, locale)}`)
      lines.push('')
      lines.push(
        locale === 'en'
          ? `- **Action:** ${textForLocale(s.action, locale)}`
          : `- **动作：** ${textForLocale(s.action, locale)}`,
      )
      lines.push(
        locale === 'en'
          ? `- **Evidence:** ${textForLocale(s.evidence, locale)}`
          : `- **证据：** ${textForLocale(s.evidence, locale)}`,
      )
      if (s.runbookPaths.length) {
        lines.push(
          locale === 'en'
            ? `- **Runbooks:** ${s.runbookPaths.map((p) => `\`${p}\``).join(', ')}`
            : `- **Runbook：** ${s.runbookPaths.map((p) => `\`${p}\``).join('、')}`,
        )
      }
      if (s.templateIds?.length) {
        lines.push(
          locale === 'en'
            ? `- **Deploy kit samples:** ${s.templateIds.map((id) => `\`${id}\``).join(', ')}`
            : `- **材料包样例：** ${s.templateIds.map((id) => `\`${id}\``).join('、')}`,
        )
      }
      lines.push(
        locale === 'en'
          ? '- **Sign-off:** owner ________  date ________  result ☐ pass ☐ fail'
          : '- **签核：** 责任人 ________  日期 ________  结果 ☐ 通过 ☐ 未通过',
      )
      lines.push('')
      n += 1
    }
  }

  lines.push(locale === 'en' ? '## Disclaimer' : '## 免责声明')
  lines.push('')
  lines.push(
    locale === 'en'
      ? buildDeployRunbookDrillSummary(locale, steps).note
      : buildDeployRunbookDrillSummary(locale, steps).note,
  )
  lines.push('')
  return lines.join('\n')
}

export function deployRunbookDrillFilename(at = new Date()): string {
  const stamp = at.toISOString().replace(/[:.]/g, '-').slice(0, 19)
  return `mts-deploy-runbook-drill-${stamp}.md`
}

/** 将联调清单追加到既有部署材料包 Markdown 末尾 */
export function appendDrillToDeployKitMarkdown(
  kitMarkdown: string,
  locale: LocaleCode = 'zh',
  steps = DEPLOY_DRILL_STEPS,
): string {
  const base = String(kitMarkdown || '').replace(/\s*$/, '')
  const drill = formatDeployRunbookDrillMarkdown(locale, steps)
  const sep =
    locale === 'en'
      ? '\n\n---\n\n## Appendix: runbook drill checklist\n\n'
      : '\n\n---\n\n## 附录：Runbook 联调清单\n\n'
  // strip the top H1 from drill to avoid double title under appendix
  const body = drill.replace(/^# .*\n+/, '')
  return `${base}${sep}${body}`
}
