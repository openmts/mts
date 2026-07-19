/** 备份/快照演练步骤（纯数据，便于单测与 Storage 页引导） */

import type { LocalizedText } from './localizedText.ts'

export type DrillSeverity = 'required' | 'recommended'

export interface BackupDrillStep {
  id: string
  severity: DrillSeverity
  title: LocalizedText
  detail: LocalizedText
  /** 可在 Dashboard 直接操作 */
  inDashboard: boolean
  routeHint?: string
}

export const BACKUP_DRILL_STEPS: BackupDrillStep[] = [
  {
    id: 'validate',
    severity: 'required',
    title: { zh: '存储校验', en: 'Storage validate' },
    detail: {
      zh: '执行 storage/validate，确认引擎 healthy/ready 与数据目录可读。',
      en: 'Run storage/validate and confirm the engine is healthy/ready and the data dir is readable.',
    },
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'snapshot',
    severity: 'required',
    title: { zh: '创建配置快照', en: 'Create config snapshot' },
    detail: {
      zh: '调用 storage/snapshot 生成配置/健康 JSON 快照（非 data_dir）。',
      en: 'Call storage/snapshot to create a config/health JSON snapshot (not data_dir).',
    },
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'data-snapshot',
    severity: 'required',
    title: { zh: '创建 data_dir 快照', en: 'Create data_dir snapshot' },
    detail: {
      zh: '调用 storage/data-snapshot（storagecheck.Snapshot）拷贝 live data_dir 到 backups。',
      en: 'Call storage/data-snapshot (storagecheck.Snapshot) to copy the live data_dir into backups.',
    },
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'copy-offbox',
    severity: 'required',
    title: { zh: '异地/旁路拷贝', en: 'Off-box / side-path copy' },
    detail: {
      zh: '将快照目录拷贝到另一磁盘或对象位置，避免与源盘同损。',
      en: 'Copy the snapshot directory to another disk or object location so it does not share fate with the source disk.',
    },
    inDashboard: false,
  },
  {
    id: 'export-config',
    severity: 'recommended',
    title: { zh: '导出配置与健康快照', en: 'Export config and health snapshot' },
    detail: {
      zh: 'storage/export 下载 JSON，便于恢复后对照配置。',
      en: 'Download JSON via storage/export for post-restore config comparison.',
    },
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'restore-side',
    severity: 'required',
    title: { zh: '旁路恢复验证', en: 'Side-path restore drill' },
    detail: {
      zh: '调用 storage/restore-drill 旁路恢复到 backups/restore-drill-* 并做 storagecheck（亦可 TestDataDirSidePathRestoreDrill）。',
      en: 'Call storage/restore-drill to restore into backups/restore-drill-* and run storagecheck (or TestDataDirSidePathRestoreDrill).',
    },
    inDashboard: true,
    routeHint: '/storage',
  },
  {
    id: 'cutover-plan',
    severity: 'recommended',
    title: { zh: '切换与回滚预案', en: 'Cutover and rollback plan' },
    detail: {
      zh: '明确 RTO/RPO、切换窗口与失败回退路径，并写入运维 runbook。',
      en: 'Define RTO/RPO, cutover window and failure rollback path; document them in the ops runbook.',
    },
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
