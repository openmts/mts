/** 可商用上线检查清单（纯数据，便于单测与文档同步） */

import type { LocalizedText } from './localizedText.ts'

export type ChecklistSeverity = 'required' | 'recommended'

export interface ProductionCheckItem {
  id: string
  severity: ChecklistSeverity
  title: LocalizedText
  detail: LocalizedText
  /** 自动化是否已覆盖 */
  automated: boolean
}

export const PRODUCTION_CHECKLIST: ProductionCheckItem[] = [
  {
    id: 'https-edge',
    severity: 'required',
    title: { zh: '边缘 HTTPS / TLS', en: 'Edge HTTPS / TLS' },
    detail: {
      zh: '边缘终止 TLS/HSTS，或启用 mts-server HTTP TLS（启用后自动发 HSTS）；见 edgeHttpsAcceptance 清单与 doctor API。',
      en: 'Terminate TLS/HSTS at the edge, or enable mts-server HTTP TLS (HSTS is emitted automatically); see edgeHttpsAcceptance and doctor API.',
    },
    automated: true,
  },
  {
    id: 'security-headers',
    severity: 'required',
    title: { zh: '安全响应头', en: 'Security response headers' },
    detail: {
      zh: 'nosniff / DENY / CSP / Referrer-Policy 等由 wrapHTTP 默认写入。',
      en: 'nosniff / DENY / CSP / Referrer-Policy are written by wrapHTTP by default.',
    },
    automated: true,
  },
  {
    id: 'change-default-admin',
    severity: 'required',
    title: { zh: '修改默认 admin 密码', en: 'Change default admin password' },
    detail: {
      zh: 'bootstrap 默认密码登录后 must_change_password 拦截业务 API，直至改密完成。',
      en: 'After bootstrap default-password login, must_change_password blocks business APIs until password change completes.',
    },
    automated: true,
  },
  {
    id: 'health-ready-metrics',
    severity: 'required',
    title: { zh: '健康与指标接入', en: 'Health and metrics integration' },
    detail: {
      zh: '/healthz /readyz /metrics 接入监控与告警；Dashboard /observability/metrics 可只读浏览。',
      en: 'Wire /healthz /readyz /metrics into monitoring/alerts; Dashboard /observability/metrics provides read-only browse.',
    },
    automated: true,
  },
  {
    id: 'backup-snapshot',
    severity: 'recommended',
    title: { zh: '备份与快照演练', en: 'Backup and snapshot drills' },
    detail: {
      zh: 'Storage 演练清单 + data-snapshot/restore-drill API + TestDataDirSidePathRestoreDrill；异地拷贝仍人工。',
      en: 'Storage drill list + data-snapshot/restore-drill API + TestDataDirSidePathRestoreDrill; off-host copy remains manual.',
    },
    automated: true,
  },
  {
    id: 'smoke-login-query-write',
    severity: 'required',
    title: {
      zh: '核心冒烟：登录-查询-写入-运维',
      en: 'Core smoke: login-query-write-ops',
    },
    detail: {
      zh: 'TestCommercialDashboardSmoke + Playwright commercial-smoke 覆盖登录/改密/写/查/运维。',
      en: 'TestCommercialDashboardSmoke + Playwright commercial-smoke cover login/password change/write/query/ops.',
    },
    automated: true,
  },
  {
    id: 'data-restore-ui',
    severity: 'recommended',
    title: { zh: 'data_dir 旁路恢复编排', en: 'data_dir side-path restore orchestration' },
    detail: {
      zh: 'Storage 页 data-snapshot + restore-drill；目标仅限 backups 下旁路目录。',
      en: 'Storage page data-snapshot + restore-drill; targets limited to side-path dirs under backups.',
    },
    automated: true,
  },
  {
    id: 'readiness-center',
    severity: 'recommended',
    title: { zh: '可商用就绪中心', en: 'Commercial readiness center' },
    detail: {
      zh: 'Dashboard /ops/readiness 聚合清单、HTTPS 验收、备份编排与 doctor；评分含 doctor warn/TLS。',
      en: 'Dashboard /ops/readiness aggregates checklist, HTTPS acceptance, backup schedule and doctor; score includes doctor warn/TLS.',
    },
    automated: true,
  },
  {
    id: 'admin-doctor',
    severity: 'recommended',
    title: { zh: '部署 Doctor API', en: 'Deploy Doctor API' },
    detail: {
      zh: 'GET /api/v1/admin/doctor + Overview 展示；CLI mts-server doctor 同口径。',
      en: 'GET /api/v1/admin/doctor + Overview display; CLI mts-server doctor uses the same contract.',
    },
    automated: true,
  },
  {
    id: 'backup-script',
    severity: 'recommended',
    title: { zh: '备份编排脚本', en: 'Backup orchestration script' },
    detail: {
      zh: 'scripts/mts-backup.sh 支持 data-snapshot / rsync / restore-drill；make backup-script-check 自检。',
      en: 'scripts/mts-backup.sh supports data-snapshot / rsync / restore-drill; make backup-script-check self-checks.',
    },
    automated: true,
  },
  {
    id: 'backup-schedule',
    severity: 'recommended',
    title: { zh: '跨主机定时备份编排', en: 'Cross-host scheduled backup orchestration' },
    detail: {
      zh: 'scripts/mts-backup.sh + 就绪中心指引 + cron/systemd 样例；实际调度在部署侧。',
      en: 'scripts/mts-backup.sh + readiness guidance + cron/systemd samples; real scheduling is deployment-side.',
    },
    automated: true,
  },
  {
    id: 'production-runbook',
    severity: 'recommended',
    title: { zh: '生产 Runbook', en: 'Production runbook' },
    detail: {
      zh: 'docs/ops/dashboard-production-runbook.md 覆盖拓扑、清单、代理与应急。',
      en: 'docs/ops/dashboard-production-runbook.md covers topology, checklist, proxy and incident response.',
    },
    automated: false,
  },
  {
    id: 'rbac-matrix-ui',
    severity: 'recommended',
    title: { zh: '权限矩阵可视化', en: 'RBAC matrix visualization' },
    detail: {
      zh: 'Dashboard /access 能力对照 + /access/grants 实时 grants 汇总。',
      en: 'Dashboard /access capability map + /access/grants live grant summary.',
    },
    automated: true,
  },
  {
    id: 'rbac-review',
    severity: 'recommended',
    title: { zh: '权限矩阵复核', en: 'RBAC matrix review' },
    detail: {
      zh: '确认非 admin 仅可访问授权库的读写能力。',
      en: 'Confirm non-admin users can only read/write granted databases.',
    },
    automated: false,
  },
]

export function requiredChecklist(items = PRODUCTION_CHECKLIST): ProductionCheckItem[] {
  return items.filter((x) => x.severity === 'required')
}

export function automatedCoverage(items = PRODUCTION_CHECKLIST): {
  total: number
  automated: number
  ratio: number
} {
  const total = items.length
  const automated = items.filter((x) => x.automated).length
  return { total, automated, ratio: total === 0 ? 0 : automated / total }
}
