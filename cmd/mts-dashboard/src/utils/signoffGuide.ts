/** 部署侧签核字段填写引导（不计入评分、不宣称验收完成） */

import type { LocaleCode } from './localizedText.ts'
import type { SignoffNoteField } from './signoffExport.ts'

export interface SignoffGuideStep {
  id: string
  title: { zh: string; en: string }
  detail: { zh: string; en: string }
}

export interface SignoffFieldGuide {
  field: SignoffNoteField
  summary: { zh: string; en: string }
  steps: SignoffGuideStep[]
  example: { zh: string; en: string }
}

export const SIGNOFF_FIELD_GUIDES: SignoffFieldGuide[] = [
  {
    field: 'edgeHttps',
    summary: {
      zh: '记录边缘 TLS/HSTS 的人工验收证据：谁验收、何时、用什么命令、结论是什么。',
      en: 'Record human evidence for edge TLS/HSTS: who, when, which command, and the conclusion.',
    },
    steps: [
      {
        id: 'edge-owner',
        title: { zh: '记录验收人与变更单', en: 'Record acceptor and change ticket' },
        detail: {
          zh: '写明验收人、日期、变更单/工单号，便于归档追溯。',
          en: 'Include acceptor, date, and change/ticket ID for audit trail.',
        },
      },
      {
        id: 'edge-probe',
        title: { zh: '执行证书与 HSTS 探测', en: 'Probe cert and HSTS' },
        detail: {
          zh: '使用 curl -I / openssl s_client 等核对证书链与 Strict-Transport-Security。',
          en: 'Use curl -I / openssl s_client to verify cert chain and Strict-Transport-Security.',
        },
      },
      {
        id: 'edge-result',
        title: { zh: '摘要结论与到期日', en: 'Summarize result and expiry' },
        detail: {
          zh: '写清证书到期日、HSTS 是否生效、是否发现阻断问题。',
          en: 'Note cert expiry, whether HSTS is effective, and any blockers.',
        },
      },
    ],
    example: {
      zh: '验收人：ops-alice；日期：2026-07-20；变更单：CHG-1024。curl -I https://mts.example.com 返回 HSTS max-age=31536000；openssl 证书链完整，到期 2027-01-15。结论：边缘 TLS/HSTS 验收通过。',
      en: 'Acceptor: ops-alice; date: 2026-07-20; ticket: CHG-1024. curl -I https://mts.example.com shows HSTS max-age=31536000; openssl chain OK, expires 2027-01-15. Result: edge TLS/HSTS accepted.',
    },
  },
  {
    field: 'backupOffsite',
    summary: {
      zh: '记录异地/跨主机备份的目标、成功时间与保留策略，证明备份不在本机孤岛。',
      en: 'Record off-host backup target, last success time, and retention so backups are not single-host islands.',
    },
    steps: [
      {
        id: 'backup-target',
        title: { zh: '写明备份目标', en: 'Specify backup target' },
        detail: {
          zh: '主机名/路径、传输方式（rsync/对象存储同步等）与账号或凭据位置（勿写密钥）。',
          en: 'Host/path, transfer method (rsync/object sync), and credential location (never paste secrets).',
        },
      },
      {
        id: 'backup-success',
        title: { zh: '记录最近一次成功', en: 'Record last success' },
        detail: {
          zh: '时间戳、耗时、数据量或 job id，证明任务真实跑通。',
          en: 'Timestamp, duration, data size or job id proving a real run.',
        },
      },
      {
        id: 'backup-retention',
        title: { zh: '保留策略与 runbook', en: 'Retention and runbook' },
        detail: {
          zh: '保留份数/天数与恢复 runbook 链接，便于交接。',
          en: 'Retention count/days and restore runbook link for handoff.',
        },
      },
    ],
    example: {
      zh: '目标：backup-b.example.com:/data/mts-offsite（rsync over ssh）。最近成功：2026-07-19 02:10 CST，约 12GB，job #8842。保留：7 日滚动。Runbook：docs/ops/dashboard-production-runbook.md#offsite-backup。密钥存于 vault path ops/mts/rsync（未写入本文）。',
      en: 'Target: backup-b.example.com:/data/mts-offsite (rsync over ssh). Last success: 2026-07-19 02:10 CST, ~12GB, job #8842. Retention: 7-day rolling. Runbook: docs/ops/dashboard-production-runbook.md#offsite-backup. Credentials in vault path ops/mts/rsync (not in this note).',
    },
  },
  {
    field: 'backupAlert',
    summary: {
      zh: '记录备份失败告警通道与最近一次演练，证明失败可被发现。',
      en: 'Record backup-failure alert channel and last drill so failures are observable.',
    },
    steps: [
      {
        id: 'alert-channel',
        title: { zh: '写明告警通道', en: 'Specify alert channel' },
        detail: {
          zh: '邮件/IM/Webhook 名称与值班组；勿写入 token。',
          en: 'Mail/IM/Webhook name and on-call group; never paste tokens.',
        },
      },
      {
        id: 'alert-drill',
        title: { zh: '记录失败演练', en: 'Record failure drill' },
        detail: {
          zh: '人为触发一次失败或 dry-run，确认告警触达与响应人。',
          en: 'Trigger a failure or dry-run and confirm delivery plus responder.',
        },
      },
      {
        id: 'alert-escalation',
        title: { zh: '升级路径', en: 'Escalation path' },
        detail: {
          zh: '超时未确认时的二级联系人/工单流程简述。',
          en: 'Brief second-line contact / ticket path when unacknowledged.',
        },
      },
    ],
    example: {
      zh: '通道：PagerDuty service mts-backup + 企业微信值班组「存储值班」。演练：2026-07-18 故意让 rsync 失败，3 分钟内 PD 与 IM 均收到；确认人 ops-bob。升级：15 分钟未 ACK 转 L2 电话。Webhook token 未写入本文。',
      en: 'Channel: PagerDuty service mts-backup + IM on-call group storage-duty. Drill: 2026-07-18 forced rsync failure; PD+IM within 3m; acked by ops-bob. Escalation: L2 call after 15m unacked. Webhook token not written here.',
    },
  },
]

const guideByField = new Map(SIGNOFF_FIELD_GUIDES.map((g) => [g.field, g]))

export function signoffGuideFor(field: SignoffNoteField): SignoffFieldGuide {
  const g = guideByField.get(field)
  if (!g) throw new Error(`unknown signoff field: ${field}`)
  return g
}

export function signoffGuideSummary(field: SignoffNoteField, locale: LocaleCode = 'zh'): string {
  const g = signoffGuideFor(field)
  return locale === 'en' ? g.summary.en : g.summary.zh
}

export function signoffGuideExample(field: SignoffNoteField, locale: LocaleCode = 'zh'): string {
  const g = signoffGuideFor(field)
  return locale === 'en' ? g.example.en : g.example.zh
}

/** 空字段时插入示例；已有内容则拒绝覆盖（返回 null） */
export function applySignoffExample(
  current: string | undefined | null,
  field: SignoffNoteField,
  locale: LocaleCode = 'zh',
  opts?: { force?: boolean },
): string | null {
  const cur = (current ?? '').trim()
  if (cur && !opts?.force) return null
  return signoffGuideExample(field, locale)
}

export function localizedSignoffGuideSteps(
  field: SignoffNoteField,
  locale: LocaleCode = 'zh',
): Array<{ id: string; title: string; detail: string }> {
  const g = signoffGuideFor(field)
  return g.steps.map((s) => ({
    id: s.id,
    title: locale === 'en' ? s.title.en : s.title.zh,
    detail: locale === 'en' ? s.detail.en : s.detail.zh,
  }))
}
