/** 备份/快照演练步骤（纯数据，便于单测与 Storage 页引导） */

export type DrillSeverity = 'required' | 'recommended'

export interface BackupDrillStep {
  id: string
  severity: DrillSeverity
  title: string
  detail: string
  /** 可在 Dashboard 直接操作 */
  inDashboard: boolean
  routeHint?: string
}

export const BACKUP_DRILL_STEPS: BackupDrillStep[] = [
  {
    id: 'validate',
    severity: 'required',
    title: '存储校验',
    detail: '执行 storage/validate，确认引擎 healthy/ready 与数据目录可读。',
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'snapshot',
    severity: 'required',
    title: '创建配置快照',
    detail: '调用 storage/snapshot 生成配置/健康 JSON 快照（非 data_dir）。',
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'data-snapshot',
    severity: 'required',
    title: '创建 data_dir 快照',
    detail: '调用 storage/data-snapshot（storagecheck.Snapshot）拷贝 live data_dir 到 backups。',
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'copy-offbox',
    severity: 'required',
    title: '异地/旁路拷贝',
    detail: '将快照目录拷贝到另一磁盘或对象位置，避免与源盘同损。',
    inDashboard: false,
  },
  {
    id: 'export-config',
    severity: 'recommended',
    title: '导出配置与健康快照',
    detail: 'storage/export 下载 JSON，便于恢复后对照配置。',
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'restore-side',
    severity: 'required',
    title: '旁路恢复验证',
    detail: '调用 storage/restore-drill 旁路恢复到 backups/restore-drill-* 并做 storagecheck（亦可 TestDataDirSidePathRestoreDrill）。',
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'cutover-plan',
    severity: 'recommended',
    title: '切换与回滚预案',
    detail: '明确 RTO/RPO、切换窗口与失败回退路径，并写入运维 runbook。',
    inDashboard: false,
  },
]

export function requiredDrillSteps(steps = BACKUP_DRILL_STEPS): BackupDrillStep[] {
  return steps.filter((s) => s.severity === 'required')
}

export function dashboardDrillSteps(steps = BACKUP_DRILL_STEPS): BackupDrillStep[] {
  return steps.filter((s) => s.inDashboard)
}

export function drillProgress(
  completedIds: string[],
  steps = BACKUP_DRILL_STEPS,
): { total: number; completed: number; requiredTotal: number; requiredCompleted: number; ratio: number } {
  const done = new Set(completedIds)
  const required = requiredDrillSteps(steps)
  const completed = steps.filter((s) => done.has(s.id)).length
  const requiredCompleted = required.filter((s) => done.has(s.id)).length
  return {
    total: steps.length,
    completed,
    requiredTotal: required.length,
    requiredCompleted,
    ratio: steps.length === 0 ? 0 : completed / steps.length,
  }
}
