/** 跨主机/定时备份编排指引（纯数据；部署侧执行，Dashboard 仅引导） */

export type BackupScheduleSeverity = 'required' | 'recommended'

export interface BackupScheduleStep {
  id: string
  severity: BackupScheduleSeverity
  title: string
  detail: string
  /** 示例命令或 cron（可复制） */
  example?: string
}

export const BACKUP_SCHEDULE_STEPS: BackupScheduleStep[] = [
  {
    id: 'define-rpo-rto',
    severity: 'required',
    title: '定义 RPO / RTO',
    detail: '明确可接受的数据丢失窗口与恢复时限，并写入运维 runbook。',
  },
  {
    id: 'local-data-snapshot',
    severity: 'required',
    title: '本地 data_dir 快照',
    detail: '使用 Storage 页 data-snapshot 或 POST /api/v1/admin/storage/data-snapshot 生成 backups/data-snapshot-*。',
    example: 'curl -X POST -H "Authorization: Bearer $TOKEN" $BASE/api/v1/admin/storage/data-snapshot -d \'{"flush":true}\'',
  },
  {
    id: 'offbox-copy',
    severity: 'required',
    title: '异地/跨主机拷贝',
    detail: '将 data-snapshot 目录同步到另一磁盘、主机或对象存储，避免与源盘同损。',
    example: 'rsync -a --delete /var/lib/mts/backups/data-snapshot-XXX/ backup-host:/backups/mts/$(date +%F)/',
  },
  {
    id: 'cron-schedule',
    severity: 'required',
    title: '定时调度',
    detail: '用 cron/systemd timer 运行仓库 scripts/mts-backup.sh；保留最近 N 份。',
    example: '15 * * * * /opt/mts/scripts/mts-backup.sh >>/var/log/mts-backup.log 2>&1',
  },
  {
    id: 'restore-drill-weekly',
    severity: 'required',
    title: '周期旁路恢复演练',
    detail: '至少每周执行 restore-drill 或旁路拉起校验，记录成功/失败与耗时。',
    example: 'curl -X POST -H "Authorization: Bearer $TOKEN" $BASE/api/v1/admin/storage/restore-drill',
  },
  {
    id: 'retention-cleanup',
    severity: 'recommended',
    title: '快照保留与清理',
    detail: '按 RPO 保留策略删除过期 data-snapshot/restore-drill，监控备份盘用量。',
    example: 'find /var/lib/mts/backups -maxdepth 1 -name "data-snapshot-*" -mtime +7 -exec rm -rf {} +',
  },
  {
    id: 'alert-on-failure',
    severity: 'recommended',
    title: '失败告警',
    detail: '备份脚本非 0 退出、restore-drill fatal、磁盘水位接入监控告警。',
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
