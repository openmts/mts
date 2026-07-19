/** 可商用上线检查清单（纯数据，便于单测与文档同步） */

export type ChecklistSeverity = 'required' | 'recommended'

export interface ProductionCheckItem {
  id: string
  severity: ChecklistSeverity
  title: string
  detail: string
  /** 自动化是否已覆盖 */
  automated: boolean
}

export const PRODUCTION_CHECKLIST: ProductionCheckItem[] = [
  {
    id: 'https-edge',
    severity: 'required',
    title: '边缘 HTTPS / TLS',
    detail: '边缘终止 TLS/HSTS，或启用 mts-server HTTP TLS（启用后自动发 HSTS）；见 edgeHttpsAcceptance 清单与 doctor API。',
    automated: true,
  },
  {
    id: 'security-headers',
    severity: 'required',
    title: '安全响应头',
    detail: 'nosniff / DENY / CSP / Referrer-Policy 等由 wrapHTTP 默认写入。',
    automated: true,
  },
  {
    id: 'change-default-admin',
    severity: 'required',
    title: '修改默认 admin 密码',
    detail: 'bootstrap 默认密码登录后 must_change_password 拦截业务 API，直至改密完成。',
    automated: true,
  },
  {
    id: 'health-ready-metrics',
    severity: 'required',
    title: '健康与指标接入',
    detail: '/healthz /readyz /metrics 接入监控与告警；Dashboard /observability/metrics 可只读浏览。',
    automated: true,
  },
  {
    id: 'backup-snapshot',
    severity: 'recommended',
    title: '备份与快照演练',
    detail: 'Storage 演练清单 + data-snapshot/restore-drill API + TestDataDirSidePathRestoreDrill；异地拷贝仍人工。',
    automated: true,
  },
  {
    id: 'smoke-login-query-write',
    severity: 'required',
    title: '核心冒烟：登录-查询-写入-运维',
    detail: 'TestCommercialDashboardSmoke + Playwright commercial-smoke 覆盖登录/改密/写/查/运维。',
    automated: true,
  },
  {
    id: 'data-restore-ui',
    severity: 'recommended',
    title: 'data_dir 旁路恢复编排',
    detail: 'Storage 页 data-snapshot + restore-drill；目标仅限 backups 下旁路目录。',
    automated: true,
  },
  {
    id: 'readiness-center',
    severity: 'recommended',
    title: '可商用就绪中心',
    detail: 'Dashboard /ops/readiness 聚合清单、HTTPS 验收、备份编排与 doctor；评分含 doctor warn/TLS。',
    automated: true,
  },
  {
    id: 'admin-doctor',
    severity: 'recommended',
    title: '部署 Doctor API',
    detail: 'GET /api/v1/admin/doctor + Overview 展示；CLI mts-server doctor 同口径。',
    automated: true,
  },
  {
    id: 'backup-script',
    severity: 'recommended',
    title: '备份编排脚本',
    detail: 'scripts/mts-backup.sh 支持 data-snapshot / rsync / restore-drill；make backup-script-check 自检。',
    automated: true,
  },
  {
    id: 'backup-schedule',
    severity: 'recommended',
    title: '跨主机定时备份编排',
    detail: 'scripts/mts-backup.sh + 就绪中心指引 + cron/systemd 样例；实际调度在部署侧。',
    automated: true,
  },
  {
    id: 'production-runbook',
    severity: 'recommended',
    title: '生产 Runbook',
    detail: 'docs/ops/dashboard-production-runbook.md 覆盖拓扑、清单、代理与应急。',
    automated: false,
  },
  {
    id: 'rbac-matrix-ui',
    severity: 'recommended',
    title: '权限矩阵可视化',
    detail: 'Dashboard /access 能力对照 + /access/grants 实时 grants 汇总。',
    automated: true,
  },
  {
    id: 'rbac-review',
    severity: 'recommended',
    title: '权限矩阵复核',
    detail: '确认非 admin 仅可访问授权库的读写能力。',
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
