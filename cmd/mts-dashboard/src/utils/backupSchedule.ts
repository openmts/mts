/** 跨主机/定时备份编排指引（纯数据；部署侧执行，Dashboard 仅引导） */

import type { LocalizedText } from './localizedText.ts'

export type BackupScheduleSeverity = 'required' | 'recommended'

export interface BackupScheduleStep {
  id: string
  severity: BackupScheduleSeverity
  title: LocalizedText
  detail: LocalizedText
  /** 示例命令或 cron（可复制） */
  example?: string
}

export const BACKUP_SCHEDULE_STEPS: BackupScheduleStep[] = [
  {
    id: 'define-rpo-rto',
    severity: 'required',
    title: { zh: '定义 RPO / RTO', en: 'Define RPO / RTO' },
    detail: {
      zh: '明确可接受的数据丢失窗口与恢复时限，并写入运维 runbook。',
      en: 'Define acceptable data-loss window and recovery time; document them in the ops runbook.',
    },
  },
  {
    id: 'local-data-snapshot',
    severity: 'required',
    title: { zh: '本地 data_dir 快照', en: 'Local data_dir snapshot' },
    detail: {
      zh: '使用 Storage 页 data-snapshot 或 POST /api/v1/admin/storage/data-snapshot 生成 backups/data-snapshot-*。',
      en: 'Use Storage page data-snapshot or POST /api/v1/admin/storage/data-snapshot to create backups/data-snapshot-*.',
    },
    example: 'curl -X POST -H "Authorization: Bearer $TOKEN" $BASE/api/v1/admin/storage/data-snapshot -d \'{"flush":true}\'',
  },
  {
    id: 'offbox-copy',
    severity: 'required',
    title: { zh: '异地/跨主机拷贝', en: 'Remote / cross-host copy' },
    detail: {
      zh: '将 data-snapshot 目录同步到另一磁盘、主机或对象存储，避免与源盘同损。',
      en: 'Sync the data-snapshot directory to another disk, host, or object store to avoid shared-fate failure.',
    },
    example: 'rsync -a --delete /var/lib/mts/backups/data-snapshot-XXX/ backup-host:/backups/mts/$(date +%F)/',
  },
  {
    id: 'cron-schedule',
    severity: 'required',
    title: { zh: '定时调度', en: 'Scheduled timer' },
    detail: {
      zh: '用 cron/systemd timer 运行仓库 scripts/mts-backup.sh；保留最近 N 份。',
      en: 'Run scripts/mts-backup.sh via cron/systemd timer; keep the latest N copies.',
    },
    example: '15 * * * * /opt/mts/scripts/mts-backup.sh >>/var/log/mts-backup.log 2>&1',
  },
  {
    id: 'restore-drill-weekly',
    severity: 'required',
    title: { zh: '周期旁路恢复演练', en: 'Periodic side-path restore drill' },
    detail: {
      zh: '至少每周执行 restore-drill 或旁路拉起校验，记录成功/失败与耗时。',
      en: 'At least weekly run restore-drill or side-path bring-up validation; record success/failure and duration.',
    },
    example: 'curl -X POST -H "Authorization: Bearer $TOKEN" $BASE/api/v1/admin/storage/restore-drill',
  },
  {
    id: 'retention-cleanup',
    severity: 'recommended',
    title: { zh: '快照保留与清理', en: 'Snapshot retention and cleanup' },
    detail: {
      zh: '按 RPO 保留策略删除过期 data-snapshot/restore-drill，监控备份盘用量。',
      en: 'Delete expired data-snapshot/restore-drill per RPO retention; monitor backup disk usage.',
    },
    example: 'find /var/lib/mts/backups -maxdepth 1 -name "data-snapshot-*" -mtime +7 -exec rm -rf {} +',
  },
  {
    id: 'alert-on-failure',
    severity: 'recommended',
    title: { zh: '失败告警', en: 'Failure alerting' },
    detail: {
      zh: '备份脚本非 0 退出、restore-drill fatal、磁盘水位接入监控告警。',
      en: 'Wire non-zero backup script exits, restore-drill fatals and disk watermarks into monitoring alerts.',
    },
  },
]

export function backupScheduleProgress(
  doneIds: string[],
  steps = BACKUP_SCHEDULE_STEPS,
): { total: number; done: number; requiredTotal: number; requiredDone: number; ratio: number } {
  const done = new Set(doneIds)
  const required = steps.filter((s) => s.severity === 'required')
  const requiredDone = required.filter((s) => done.has(s.id)).length
  const doneCount = steps.filter((s) => done.has(s.id)).length
  return {
    total: steps.length,
    done: doneCount,
    requiredTotal: required.length,
    requiredDone,
    ratio: steps.length === 0 ? 0 : doneCount / steps.length,
  }
}
